package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// FileEntry represents a single file or folder in the tree
type FileEntry struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Children []FileEntry `json:"children,omitempty"`
}

// Settings holds user-configurable preferences
type Settings struct {
	FontSize int    `json:"fontSize"`
	Theme    string `json:"theme"`
	Shell    string `json:"shell"`
}

// App struct
type App struct {
	ctx              context.Context
	ptyTerm          pty.Pty
	ptyProc          *pty.Cmd
	cwd              string
	settings         Settings
	commandStartTime time.Time
	commandPending   bool
	mu               sync.Mutex
}

// configDir returns the path to %APPDATA%/termi/, creating it if needed
func configDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, "termi")
	os.MkdirAll(dir, 0755)
	return dir
}

func defaultSettings() Settings {
	return Settings{
		FontSize: 14,
		Theme:    "dark",
		Shell:    "PowerShell",
	}
}

// NewApp creates a new App application struct
func NewApp() *App {
	initialDir, err := os.Getwd()
	if err != nil {
		initialDir, _ = os.UserHomeDir()
	}
	return &App{cwd: initialDir, settings: defaultSettings()}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.settings = a.LoadSettings()
}

// ---------- SETTINGS ----------

// LoadSettings reads settings from disk, returning defaults if missing
func (a *App) LoadSettings() Settings {
	path := filepath.Join(configDir(), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultSettings()
	}
	s := defaultSettings()
	json.Unmarshal(data, &s)
	return s
}

// SaveSettings writes settings to disk and updates in-memory copy
func (a *App) SaveSettings(s Settings) error {
	a.settings = s
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0644)
}

// shellCommand returns the executable and args for the selected shell
func (a *App) shellCommand() (string, []string) {
	switch a.settings.Shell {
	case "CMD":
		return "cmd.exe", nil
	case "WSL":
		return "wsl.exe", nil
	default:
		return "powershell.exe", []string{"-NoLogo", "-ExecutionPolicy", "RemoteSigned"}
	}
}

// StartTerminal initializes the Windows ConPTY session
func (a *App) StartTerminal() error {
	shellExe, shellArgs := a.shellCommand()
	fmt.Printf("Starting %s via ConPTY...\n", shellExe)

	ptyTerm, err := pty.New()
	if err != nil {
		fmt.Println("Error starting pty:", err)
		return err
	}

	cmd := ptyTerm.Command(shellExe, shellArgs...)
	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting command inside pty:", err)
		return err
	}

	a.ptyTerm = ptyTerm
	a.ptyProc = cmd

	// Emit the initial cwd immediately so the explorer panel shows the right folder
	go func() {
		runtime.EventsEmit(a.ctx, "cwd-change", a.cwd)
	}()

	// Goroutine to continuously read stdout/stderr from the PTY and pipe it to Svelte
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptyTerm.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Println("PTY Read Error:", err)
				break
			}
			chunk := string(buf[:n])
			runtime.EventsEmit(a.ctx, "terminal-output", chunk)
			a.detectCwdChange(chunk)
		}
	}()

	return nil
}

// RestartTerminal tears down the current PTY and starts a new one
func (a *App) RestartTerminal() error {
	if a.ptyTerm != nil {
		a.ptyTerm.Close()
		a.ptyTerm = nil
	}
	return a.StartTerminal()
}

// detectCwdChange parses PTY output to detect PowerShell prompt and extract cwd
func (a *App) detectCwdChange(chunk string) {
	lines := strings.Split(chunk, "\n")
	promptDetected := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// PowerShell prompt looks like: PS C:\Users\foo>
		if strings.HasPrefix(line, "PS ") && strings.Contains(line, ">") {
			start := 3
			end := strings.LastIndex(line, ">")
			if end > start {
				candidate := strings.TrimSpace(line[start:end])
				if len(candidate) >= 2 && candidate[1] == ':' {
					promptDetected = true
					if candidate != a.cwd {
						a.cwd = candidate
						runtime.EventsEmit(a.ctx, "cwd-change", a.cwd)
					}
				}
			}
		}
		// CMD prompt looks like: C:\Users\foo>
		if len(line) >= 3 && line[1] == ':' && line[2] == '\\' && strings.HasSuffix(line, ">") {
			candidate := strings.TrimSuffix(line, ">")
			if len(candidate) >= 2 {
				promptDetected = true
				if candidate != a.cwd {
					a.cwd = candidate
					runtime.EventsEmit(a.ctx, "cwd-change", a.cwd)
				}
			}
		}
	}

	// Toast: if a prompt reappeared and a command was pending, check duration
	if promptDetected {
		a.mu.Lock()
		if a.commandPending {
			elapsed := time.Since(a.commandStartTime).Seconds()
			a.commandPending = false
			a.mu.Unlock()
			if elapsed > 10 {
				runtime.EventsEmit(a.ctx, "command-complete", elapsed)
			}
		} else {
			a.mu.Unlock()
		}
	}
}

// WriteToTerminal takes input from the Svelte UI and pipes it to the actual PowerShell PTY
func (a *App) WriteToTerminal(input string) {
	if a.ptyTerm != nil {
		// Track command start time for toast notifications
		if strings.Contains(input, "\r") || strings.Contains(input, "\n") {
			a.mu.Lock()
			a.commandStartTime = time.Now()
			a.commandPending = true
			a.mu.Unlock()
		}
		a.ptyTerm.Write([]byte(input))
	}
}

// ResizeTerminal resizes the PTY to match the xterm.js dimensions
func (a *App) ResizeTerminal(cols int, rows int) {
	if a.ptyTerm != nil {
		a.ptyTerm.Resize(cols, rows)
	}
}

// GetWorkingDir returns the current tracked working directory
func (a *App) GetWorkingDir() string {
	return a.cwd
}

// ListDirectory returns one level of entries for a given path
func (a *App) ListDirectory(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []FileEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fullPath := filepath.Join(path, e.Name())
		entry := FileEntry{
			Name:  e.Name(),
			Path:  fullPath,
			IsDir: e.IsDir(),
		}
		result = append(result, entry)
	}
	return result, nil
}

// ---------- FILE OPERATIONS ----------

// OpenFileInEditor opens a file with the system default application
func (a *App) OpenFileInEditor(path string) error {
	return exec.Command("cmd", "/c", "start", "", path).Start()
}

// RenameFile renames a file or directory
func (a *App) RenameFile(oldPath string, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// DeleteFile removes a single file
func (a *App) DeleteFile(path string) error {
	return os.Remove(path)
}

// DeleteDirectory removes a directory and all contents
func (a *App) DeleteDirectory(path string) error {
	return os.RemoveAll(path)
}

// CreateDirectory creates a new directory at the given path
func (a *App) CreateDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// ---------- COMMAND HISTORY ----------

func historyPath() string {
	return filepath.Join(configDir(), "history.txt")
}

// SaveCommandHistory appends a command to the history file
func (a *App) SaveCommandHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	f, err := os.OpenFile(historyPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(cmd + "\n")
}

// LoadCommandHistory reads commands from the history file (most recent first, max 500)
func (a *App) LoadCommandHistory() []string {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Deduplicate keeping last occurrence, reverse for most-recent-first
	seen := make(map[string]bool)
	var result []string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		result = append(result, line)
		if len(result) >= 500 {
			break
		}
	}
	return result
}

// ClearCommandHistory truncates the history file
func (a *App) ClearCommandHistory() {
	os.WriteFile(historyPath(), []byte{}, 0644)
}

// ---------- AUTOCOMPLETE ----------

// GetCompletions returns completion suggestions for a partial input
func (a *App) GetCompletions(partial string, cwd string) []string {
	partial = strings.TrimSpace(partial)
	if partial == "" {
		return []string{}
	}
	lower := strings.ToLower(partial)

	seen := make(map[string]bool)
	var results []string

	addIfMatch := func(s string) {
		sl := strings.ToLower(s)
		if strings.HasPrefix(sl, lower) && !seen[sl] {
			seen[sl] = true
			results = append(results, s)
		}
	}

	// 1. History matches
	history := a.LoadCommandHistory()
	for _, h := range history {
		addIfMatch(h)
		if len(results) >= 10 {
			break
		}
	}

	// 2. Common commands
	commonCmds := []string{
		"dir", "cd", "cls", "exit", "echo", "type", "mkdir", "rmdir", "del", "copy", "move", "ren",
		"git", "git status", "git add", "git commit", "git push", "git pull", "git log", "git diff",
		"npm", "npm install", "npm run", "npm start", "npm test",
		"go", "go build", "go run", "go test", "go mod tidy",
		"code", "code .", "python", "node", "pip install",
		"ls", "cat", "grep", "curl", "wget", "ssh", "docker", "kubectl",
	}
	for _, c := range commonCmds {
		if len(results) >= 10 {
			break
		}
		addIfMatch(c)
	}

	// 3. File/directory names in cwd
	if len(results) < 10 && cwd != "" {
		entries, err := os.ReadDir(cwd)
		if err == nil {
			for _, e := range entries {
				if len(results) >= 10 {
					break
				}
				addIfMatch(e.Name())
			}
		}
	}

	sort.Strings(results)
	if len(results) > 10 {
		results = results[:10]
	}
	return results
}
