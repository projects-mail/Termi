package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// TerminalEvent is used to send JSON data to frontend with TabID
type TerminalEvent struct {
	TabID string `json:"tabId"`
	Data  string `json:"data"`
}

// TerminalSession holds data for a single tab
type TerminalSession struct {
	ID               string
	PtyTerm          pty.Pty
	PtyCmd           *pty.Cmd
	Cwd              string
	CommandStartTime time.Time
	CommandPending   bool
}

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
	ctx      context.Context
	sessions map[string]*TerminalSession
	settings Settings
	mu       sync.Mutex
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
	return &App{
		sessions: make(map[string]*TerminalSession),
		settings: defaultSettings(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.settings = a.LoadSettings()
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
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
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0644)
}

// shellCommand returns the executable and args for the selected shell
func (a *App) shellCommand() (string, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	exe := "powershell.exe"
	var args []string
	
	switch a.settings.Shell {
	case "CMD":
		exe = "cmd.exe"
	case "WSL":
		exe = "wsl.exe"
	default:
		exe = "powershell.exe"
		args = []string{"-NoLogo", "-ExecutionPolicy", "RemoteSigned"}
	}
	
	if fullPath, err := exec.LookPath(exe); err == nil {
		exe = fullPath
	}
	
	return exe, args
}

// ---------- MULTI-TAB TERMINAL OPS ----------

// StartTerminal initializes a new Windows ConPTY session and returns its tab ID.
// If initialDir is provided, the shell starts there (if supported by the shell).
func (a *App) StartTerminal(initialDir string) (string, error) {
	if initialDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			initialDir, _ = os.UserHomeDir()
		} else {
			initialDir = cwd
		}
	}

	shellExe, shellArgs := a.shellCommand()
	fmt.Printf("Starting %s via ConPTY...\n", shellExe)

	ptyTerm, err := pty.New()
	if err != nil {
		fmt.Println("Error starting pty:", err)
		return "", err
	}

	cmd := ptyTerm.Command(shellExe, shellArgs...)
	cmd.Dir = initialDir

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting command inside pty:", err)
		return "", err
	}

	sessionID := generateID()
	session := &TerminalSession{
		ID:      sessionID,
		PtyTerm: ptyTerm,
		PtyCmd:  cmd,
		Cwd:     initialDir,
	}

	a.mu.Lock()
	a.sessions[sessionID] = session
	a.mu.Unlock()

	// Emit initial CWD immediately
	go func() {
		runtime.EventsEmit(a.ctx, "cwd-change", TerminalEvent{
			TabID: sessionID,
			Data:  initialDir,
		})
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
				fmt.Println("PTY Read Error [", sessionID, "]:", err)
				break
			}
			chunk := string(buf[:n])
			
			// Broadcast output
			runtime.EventsEmit(a.ctx, "terminal-output", TerminalEvent{
				TabID: sessionID,
				Data:  chunk,
			})
			
			// Detect CWD
			a.detectCwdChange(sessionID, chunk)
		}
		
		// When PTY closes naturally (e.g. user typed 'exit')
		runtime.EventsEmit(a.ctx, "terminal-closed", sessionID)
		a.CloseTerminal(sessionID)
	}()

	return sessionID, nil
}

// CloseTerminal cleanly stops a PTY session
func (a *App) CloseTerminal(tabID string) error {
	a.mu.Lock()
	session, exists := a.sessions[tabID]
	if exists {
		delete(a.sessions, tabID)
	}
	a.mu.Unlock()

	if exists && session.PtyTerm != nil {
		session.PtyTerm.Close()
	}
	return nil
}

// RestartTerminal tears down the current PTY and starts a new one with the same ID logic
func (a *App) RestartTerminal(tabID string) (string, error) {
	a.CloseTerminal(tabID)
	return a.StartTerminal("")
}

// detectCwdChange parses PTY output to detect PowerShell prompt and extract cwd
func (a *App) detectCwdChange(tabID string, chunk string) {
	a.mu.Lock()
	session, exists := a.sessions[tabID]
	if !exists {
		a.mu.Unlock()
		return
	}
	cwdCache := session.Cwd
	var cmdStartTime time.Time
	cmdPending := session.CommandPending
	a.mu.Unlock()

	lines := strings.Split(chunk, "\n")
	promptDetected := false
	newCwd := cwdCache

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
					newCwd = candidate
				}
			}
		}
		// CMD prompt looks like: C:\Users\foo>
		if len(line) >= 3 && line[1] == ':' && line[2] == '\\' && strings.HasSuffix(line, ">") {
			candidate := strings.TrimSuffix(line, ">")
			if len(candidate) >= 2 {
				promptDetected = true
				newCwd = candidate
			}
		}
	}

	a.mu.Lock()
	if promptDetected && session.Cwd != newCwd {
		session.Cwd = newCwd
		runtime.EventsEmit(a.ctx, "cwd-change", TerminalEvent{
			TabID: tabID,
			Data:  newCwd,
		})
	}

	// Toast: if a prompt reappeared and a command was pending, check duration
	if promptDetected && cmdPending {
		cmdStartTime = session.CommandStartTime
		session.CommandPending = false
		elapsed := time.Since(cmdStartTime).Seconds()
		if elapsed > 10 {
			runtime.EventsEmit(a.ctx, "command-complete", elapsed) // Could add tabID here if we want tab-specifc toasts
		}
	}
	a.mu.Unlock()
}

// WriteToTerminal takes input from the Svelte UI and pipes it to the actual PTY
func (a *App) WriteToTerminal(tabID string, input string) {
	a.mu.Lock()
	session, exists := a.sessions[tabID]
	if exists {
		// Track command start time for toast notifications
		if strings.Contains(input, "\r") || strings.Contains(input, "\n") {
			session.CommandStartTime = time.Now()
			session.CommandPending = true
		}
	}
	a.mu.Unlock()

	if exists && session.PtyTerm != nil {
		session.PtyTerm.Write([]byte(input))
	}
}

// ResizeTerminal resizes the PTY to match the xterm.js dimensions
func (a *App) ResizeTerminal(tabID string, cols int, rows int) {
	a.mu.Lock()
	session, exists := a.sessions[tabID]
	a.mu.Unlock()
	
	if exists && session.PtyTerm != nil {
		session.PtyTerm.Resize(cols, rows)
	}
}

// GetWorkingDir returns the current tracked working directory for a tab
func (a *App) GetWorkingDir(tabID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if session, exists := a.sessions[tabID]; exists {
		return session.Cwd
	}
	return ""
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

// LoadCommandHistory reads commands from the history file
func (a *App) LoadCommandHistory() []string {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
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

	history := a.LoadCommandHistory()
	for _, h := range history {
		addIfMatch(h)
		if len(results) >= 10 {
			break
		}
	}

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
