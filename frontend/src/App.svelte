<script>
import { onMount, tick } from "svelte";
    import { Terminal } from "xterm";
    import { FitAddon } from "xterm-addon-fit";
    import { SearchAddon } from "xterm-addon-search";
    import "xterm/css/xterm.css";
    import {
        StartTerminal,
        CloseTerminal,
        RestartTerminal,
        WriteToTerminal,
        ResizeTerminal,
        ListDirectory,
        GetWorkingDir,
        SaveCommandHistory,
        LoadCommandHistory,
        CreateDirectory,
    } from "../wailsjs/go/main/App.js";
    import {
        EventsOn,
        ClipboardGetText,
        ClipboardSetText,
    } from "../wailsjs/runtime/runtime.js";

    import {
        historyOpen,
        searchOpen,
        commandHistory,
    } from "./stores.js";
    import { registerShortcuts } from "./shortcuts.js";

    import TreeNode from "./TreeNode.svelte";
    import SearchBar from "./SearchBar.svelte";
    import HistoryPanel from "./HistoryPanel.svelte";
    import ContextMenu from "./ContextMenu.svelte";
    import Toast from "./Toast.svelte";
    import Autocomplete from "./Autocomplete.svelte";

    // Multi-tab state
    let tabs = [];
    let activeTabId = null;
    let tabCounter = 1;
    let explorerVisible = true;
    let searchAddon; // We might need one per tab, or just use active tab's addon
    let toastRef;

    let commandInput = "";
    let commandTextarea;

    // File explorer state
    let cwd = "";
    let cwdEditPath = "";
    let cwdEditing = false;
    let cwdEntries = [];
    let explorerLoading = false;
    let filterText = "";

    // Navigation history (back/forward)
    let navBack = [];
    let navForward = [];
    let navUserAction = false; // true when navigating via back/forward/breadcrumb (not terminal cd)

    // Pin mode — when pinned, explorer is completely frozen
    let pinned = true;

    // When navigating toward root, this is set to the previous cwd so the tree
    // auto-expands to show where you came from
    let expandToPath = "";

    // Context menu state
    let ctxMenuVisible = false;
    let ctxMenuX = 0;
    let ctxMenuY = 0;
    let ctxMenuItems = [];

    // Drag & drop
    let dragOver = false;

    // Autocomplete
    let autocompleteRef;
    let autocompleteVisible = false;

    // Filtered entries for explorer
    $: filteredEntries = filterText
        ? cwdEntries.filter((e) =>
              e.name.toLowerCase().includes(filterText.toLowerCase()),
          )
        : cwdEntries;

    // Xterm theme
    const termTheme = {
        background: "#1a1a2e",
        foreground: "#e0e0e0",
        cursor: "#00d4ff",
        selectionBackground: "#264f78",
        black: "#1a1a2e",
        brightBlack: "#444",
        red: "#f07178",
        green: "#c3e88d",
        yellow: "#ffcb6b",
        blue: "#82aaff",
        magenta: "#c792ea",
        cyan: "#89ddff",
        white: "#e0e0e0",
    };

    async function loadExplorer(path) {
        explorerLoading = true;
        try {
            cwdEntries = await ListDirectory(path);
        } catch (e) {
            cwdEntries = [];
        } finally {
            explorerLoading = false;
        }
    }

    // New folder state
    let creatingFolder = false;
    let newFolderName = "";

    function startCreateFolder() {
        creatingFolder = true;
        newFolderName = "";
    }

    async function commitCreateFolder(e) {
        if (e && e.key === "Escape") {
            creatingFolder = false;
            return;
        }
        if (e && e.key !== "Enter") return;
        const name = newFolderName.trim();
        creatingFolder = false;
        if (!name || !cwd) return;
        const folderPath = cwd + "\\" + name;
        await CreateDirectory(folderPath);
        await loadExplorer(cwd);
    }

    function cancelCreateFolder() {
        creatingFolder = false;
    }





    async function createNewTab(focus = true) {
        try {
            const tabId = await StartTerminal("");
            const term = new Terminal({
                cursorBlink: true,
                fontFamily: 'Consolas, "Courier New", monospace',
                fontSize: 14,
                theme: termTheme,
            });
            const fitAddon = new FitAddon();
            const sa = new SearchAddon();
            term.loadAddon(fitAddon);
            term.loadAddon(sa);

            const newTab = {
                id: tabId,
                name: `Terminal ${tabCounter++}`,
                cwd: "",
                term,
                fitAddon,
                searchAddon: sa,
                containerRef: null
            };

            tabs = [...tabs, newTab];
            
            term.onData((data) => {
                WriteToTerminal(tabId, data);
            });
            term.onResize(({ cols, rows }) => {
                ResizeTerminal(tabId, cols, rows);
            });
            
            term.onSelectionChange(() => {
                const sel = term.getSelection();
                if (sel && sel.trim().length > 0) {
                    ClipboardSetText(sel);
                    if (toastRef) toastRef.showToast("Copied to clipboard!");
                }
            });
            
            if (focus) {
                await switchTab(tabId);
            }
            
            // The initial cwd-change event from Go may have fired before tabs was updated.
            // Explicitly fetch the working directory here to initialize the explorer correctly.
            const initialCwd = await GetWorkingDir(tabId);
            if (initialCwd) {
                const updatedTab = tabs.find(t => t.id === tabId);
                if (updatedTab) {
                    updatedTab.cwd = initialCwd;
                    if (activeTabId === tabId && explorerVisible) {
                        cwd = initialCwd;
                        await loadExplorer(cwd);
                    }
                }
            }
        } catch(e) {
            console.error("Failed to create terminal tab", e);
        }
    }

    async function switchTab(tabId) {
        activeTabId = tabId;
        const tab = tabs.find(t => t.id === tabId);
        if (tab) {
            searchAddon = tab.searchAddon;
            
            await tick();
            
            if (tab.containerRef && !tab.term.element) {
                tab.term.open(tab.containerRef);
            }
            if (tab.fitAddon) {
                tab.fitAddon.fit();
                ResizeTerminal(tabId, tab.term.cols, tab.term.rows);
            }
            tab.term.focus();
            
            // Update Explorer
            cwd = tab.cwd;
            if (cwd && explorerVisible) loadExplorer(cwd);
        }
    }

    async function closeTab(tabId, e) {
        if(e) e.stopPropagation();
        await CloseTerminal(tabId);
        const idx = tabs.findIndex(t => t.id === tabId);
        tabs = tabs.filter(t => t.id !== tabId);
        
        if (activeTabId === tabId) {
            if (tabs.length > 0) {
                switchTab(tabs[Math.max(0, idx - 1)].id);
            } else {
                await createNewTab();
            }
        }
    }

    onMount(async () => {
        // Load command history
        const hist = await LoadCommandHistory();
        commandHistory.set(hist || []);

        window.addEventListener("resize", () => {
            const tab = tabs.find(t => t.id === activeTabId);
            if (tab && tab.fitAddon) {
                tab.fitAddon.fit();
                ResizeTerminal(activeTabId, tab.term.cols, tab.term.rows);
            }
        });

        // Pipe PTY output from Go → xterm.js
        EventsOn("terminal-output", (payload) => {
            const tab = tabs.find(t => t.id === payload.tabId);
            if (tab) {
                tab.term.write(payload.data);
            }
        });

        // Listen for cwd changes detected by Go (terminal cd)
        EventsOn("cwd-change", async (payload) => {
            const tab = tabs.find(t => t.id === payload.tabId);
            if (tab) {
                tab.cwd = payload.data;
                // If it's the active tab, synchronize explorer
                if (tab.id === activeTabId) {
                    if (pinned && cwd) return; // if pinned, UI ignores terminal cd
                    
                    if (payload.data !== cwd) {
                        if (!navUserAction && cwd) {
                            navBack = [...navBack, cwd];
                            navForward = [];
                        }
                        navUserAction = false;
                        cwd = payload.data;
                    }
                    filterText = "";
                    if (explorerVisible) await loadExplorer(cwd);
                }
            }
        });

        // --- KEYBOARD SHORTCUTS ---
        registerShortcuts({
            clearTerminal: () => {
                if(activeTabId) WriteToTerminal(activeTabId, "cls\r");
            },
            hasSelection: () => {
                const sel = tabs.find(t=>t.id===activeTabId)?.term.getSelection();
                return sel && sel.length > 0;
            },
            clearSelection: () => {
                tabs.find(t=>t.id===activeTabId)?.term.clearSelection();
            },
            copy: () => {
                const sel = tabs.find(t=>t.id===activeTabId)?.term.getSelection();
                if (sel) {
                    ClipboardSetText(sel);
                    if (toastRef) toastRef.showToast("Copied to clipboard!");
                }
            },
            paste: async () => {
                const text = await ClipboardGetText();
                if (text) if(activeTabId) WriteToTerminal(activeTabId, text);
            },
            toggleHistory: () => historyOpen.update((v) => !v),
            toggleSearch: () => searchOpen.update((v) => !v),
        });

        // Start initial Tab
        await createNewTab(true);
    });

    // --- COMMAND BAR ---
    function runCommand() {
        const cmd = commandInput.trim();
        if (!cmd) return;
        if(activeTabId) WriteToTerminal(activeTabId, cmd + "\r");

        // Save to history
        SaveCommandHistory(cmd);
        commandHistory.update((h) => {
            const filtered = h.filter((c) => c !== cmd);
            return [cmd, ...filtered];
        });

        commandInput = "";
        autocompleteVisible = false;
        tabs.find(t=>t.id===activeTabId)?.tabs.find(t=>t.id===activeTabId)?.term.focus();
    }

    function handleKeydown(e) {
        // Let autocomplete handle navigation keys first
        if (autocompleteRef && autocompleteRef.handleKeydown(e)) {
            return;
        }
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            runCommand();
        }
    }

    function handleAutocompleteSelect(e) {
        commandInput = e.detail;
        autocompleteVisible = false;
        if (commandTextarea) commandTextarea.focus();
    }

    // --- EXPLORER NAVIGATION ---
    // Checks if target is closer to root than current cwd
    function isTowardRoot(target, current) {
        const t = target.replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
        const c = current.replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
        return c.startsWith(t + "/") || c === t || t.length < c.length;
    }

    // Navigate explorer to a new root path. Re-roots the tree.
    // If going toward root, sets expandToPath so tree auto-expands to previous location.
    // When pinned, explorer navigates freely but does NOT send cd to terminal.
    async function navigateExplorer(newPath, { pushHistory = true, focusPath = "" } = {}) {
        if (newPath === cwd) return;

        if (pushHistory && cwd) {
            navBack = [...navBack, cwd];
            navForward = [];
        }

        // If going toward root, auto-expand to show where we came from
        if (focusPath) {
            expandToPath = focusPath;
        } else if (isTowardRoot(newPath, cwd)) {
            expandToPath = cwd;
        } else {
            expandToPath = "";
        }

        cwd = newPath;
        filterText = "";

        // Only cd in terminal when NOT pinned
        if (!pinned) {
            navUserAction = true;
            if(activeTabId) WriteToTerminal(activeTabId, 'cd "' + newPath + '"\r');
        }

        await loadExplorer(newPath);

        if (expandToPath) {
            setTimeout(() => { expandToPath = ""; }, 3000);
        }
    }

    function goBack() {
        if (navBack.length === 0) return;
        const prev = navBack[navBack.length - 1];
        navBack = navBack.slice(0, -1);
        navForward = [...navForward, cwd];

        expandToPath = isTowardRoot(prev, cwd) ? cwd : "";
        cwd = prev;
        filterText = "";

        if (!pinned) {
            navUserAction = true;
            if(activeTabId) WriteToTerminal(activeTabId, 'cd "' + prev + '"\r');
        }

        loadExplorer(prev);
        if (expandToPath) {
            setTimeout(() => { expandToPath = ""; }, 3000);
        }
    }

    function goForward() {
        if (navForward.length === 0) return;
        const next = navForward[navForward.length - 1];
        navForward = navForward.slice(0, -1);
        navBack = [...navBack, cwd];

        expandToPath = isTowardRoot(next, cwd) ? cwd : "";
        cwd = next;
        filterText = "";

        if (!pinned) {
            navUserAction = true;
            if(activeTabId) WriteToTerminal(activeTabId, 'cd "' + next + '"\r');
        }

        loadExplorer(next);
        if (expandToPath) {
            setTimeout(() => { expandToPath = ""; }, 3000);
        }
    }

    function goUp() {
        if (!cwd) return;
        const parts = cwd.replace(/\\/g, "/").split("/").filter(Boolean);
        if (parts.length <= 1) return;
        const parent = parts.slice(0, -1).join("\\");
        const parentPath = parent.length === 2 && parent[1] === ":" ? parent + "\\" : parent;
        navigateExplorer(parentPath);
    }

    // --- BREADCRUMB PATH ---
    function getBreadcrumbs(p) {
        if (!p) return [];
        const parts = p.replace(/\\/g, "/").split("/").filter(Boolean);
        return parts.map((part, i) => {
            const fullPath = parts.slice(0, i + 1).join("\\");
            const path = fullPath.length === 2 && fullPath[1] === ":" ? fullPath + "\\" : fullPath;
            return { label: part, path };
        });
    }

    $: breadcrumbs = getBreadcrumbs(cwd);

    function navigateToBreadcrumb(path) {
        navigateExplorer(path);
    }

    // Double-click breadcrumb bar -> copy full path
    function copyBreadcrumbPath() {
        if (!cwd) return;
        ClipboardSetText(cwd).then(() => {
            copied = true;
            setTimeout(() => (copied = false), 2000);
        });
    }

    // --- CWD PATH EDITING ---
    function startEditPath() {
        cwdEditPath = cwd;
        cwdEditing = true;
    }

    async function commitPathEdit(e) {
        if (e && e.key !== "Enter") return;
        const newPath = cwdEditPath.trim();
        cwdEditing = false;
        if (newPath && newPath !== cwd) {
            navigateExplorer(newPath);
        }
    }

    function cancelPathEdit() {
        cwdEditing = false;
    }

    // --- RESIZER & EXPLORER STATE ---
    let explorerWidth = 240;
    let isResizing = false;
    let copied = false;

    function copyPath(event) {
        event.stopPropagation();
        if (!cwd) return;
        ClipboardSetText(cwd).then(() => {
            copied = true;
            setTimeout(() => (copied = false), 2000);
        });
    }

    function startResize(e) {
        isResizing = true;
        window.addEventListener("mousemove", handleMouseMove);
        window.addEventListener("mouseup", stopResize);
        document.body.style.userSelect = "none";
        document.body.style.cursor = "col-resize";
    }

    function handleMouseMove(e) {
        if (!isResizing) return;
        const minWidth = window.innerWidth * 0.15;
        const maxWidth = window.innerWidth * 0.6;
        let newWidth = window.innerWidth - e.clientX;
        if (newWidth < minWidth) newWidth = minWidth;
        if (newWidth > maxWidth) newWidth = maxWidth;
        explorerWidth = newWidth;
        const tab = tabs.find(t => t.id === activeTabId);
        if (tab && tab.fitAddon) tab.fitAddon.fit();
    }

    function stopResize() {
        isResizing = false;
        window.removeEventListener("mousemove", handleMouseMove);
        window.removeEventListener("mouseup", stopResize);
        document.body.style.userSelect = "auto";
        document.body.style.cursor = "default";
        const tab = tabs.find(t => t.id === activeTabId);
        if (tab && tab.fitAddon) {
            tab.fitAddon.fit();
            ResizeTerminal(activeTabId, tab.term.cols, tab.term.rows);
        }
    }

    // --- TERMINAL CONTEXT MENU ---
    function handleTerminalContext(e) {
        e.preventDefault();
        ctxMenuX = e.clientX;
        ctxMenuY = e.clientY;
        ctxMenuItems = [
            {
                label: "Copy",
                shortcut: "Ctrl+Shift+C",
                action: () => {
                    const sel = tabs.find(t=>t.id===activeTabId)?.term.getSelection();
                    if (sel) ClipboardSetText(sel);
                },
            },
            {
                label: "Paste",
                shortcut: "Ctrl+Shift+V",
                action: async () => {
                    const text = await ClipboardGetText();
                    if (text) if(activeTabId) WriteToTerminal(activeTabId, text);
                },
            },
            { divider: true },
            {
                label: "Select All",
                action: () => tabs.find(t=>t.id===activeTabId)?.term.selectAll(),
            },
            {
                label: "Clear Terminal",
                shortcut: "Ctrl+L",
                action: () => WriteToTerminal("cls\r"),
            },
        ];
        ctxMenuVisible = true;
    }

    // --- FILE CONTEXT MENU ---
    function handleFileContext(e) {
        const { x, y, entry } = e.detail;
        ctxMenuX = x;
        ctxMenuY = y;

        if (entry.isDir) {
            ctxMenuItems = [
                {
                    label: "Open in Terminal",
                    action: () => {
                        if(activeTabId) WriteToTerminal(activeTabId, 'cd "' + entry.path + '"\r');
                    }
                },
                {
                    label: "Copy Path",
                    action: () => ClipboardSetText(entry.path),
                },
                { divider: true },
                {
                    label: "Rename",
                    action: () => startRename(entry),
                },
                {
                    label: "Delete",
                    action: () => confirmDelete(entry),
                },
            ];
        } else {
            ctxMenuItems = [
                {
                    label: "Open",
                    action: async () => {
                        const { OpenFileInEditor } = await import(
                            "../wailsjs/go/main/App.js"
                        );
                        OpenFileInEditor(entry.path);
                    },
                },
                {
                    label: "Open in Terminal",
                    action: () => {
                        const dir = entry.path
                            .split("\\")
                            .slice(0, -1)
                            .join("\\");
                        if(activeTabId) WriteToTerminal(activeTabId, 'cd "' + dir + '"\r');
                    },
                },
                {
                    label: "Copy Path",
                    action: () => ClipboardSetText(entry.path),
                },
                { divider: true },
                {
                    label: "Rename",
                    action: () => startRename(entry),
                },
                {
                    label: "Delete",
                    action: () => confirmDelete(entry),
                },
            ];
        }
        ctxMenuVisible = true;
    }

    // Rename state
    let renamingPath = "";
    let renameNewName = "";

    function startRename(entry) {
        renamingPath = entry.path;
        renameNewName = entry.name;
    }

    async function confirmRename(entry) {
        if (!renameNewName.trim() || renameNewName === entry.name) {
            renamingPath = "";
            return;
        }
        const { RenameFile } = await import("../wailsjs/go/main/App.js");
        const dir = entry.path.split("\\").slice(0, -1).join("\\");
        const newPath = dir + "\\" + renameNewName.trim();
        await RenameFile(entry.path, newPath);
        renamingPath = "";
        await loadExplorer(cwd);
    }

    function cancelRename() {
        renamingPath = "";
    }

    async function confirmDelete(entry) {
        // Simple confirmation via the context — just delete
        if (entry.isDir) {
            const { DeleteDirectory } = await import(
                "../wailsjs/go/main/App.js"
            );
            await DeleteDirectory(entry.path);
        } else {
            const { DeleteFile } = await import(
                "../wailsjs/go/main/App.js"
            );
            await DeleteFile(entry.path);
        }
        await loadExplorer(cwd);
    }

    // --- DRAG & DROP ---
    function handleDragOver(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = "copy";
        dragOver = true;
    }

    function handleDragLeave() {
        dragOver = false;
    }

    function handleDrop(e) {
        e.preventDefault();
        dragOver = false;
        const path = e.dataTransfer.getData("text/plain");
        if (path) {
            if(activeTabId) WriteToTerminal(activeTabId, '"' + path + '"');
            tabs.find(t=>t.id===activeTabId)?.term.focus();
        }
    }

    // --- HISTORY SELECT ---
    function handleHistorySelect(e) {
        commandInput = e.detail;
        historyOpen.set(false);
        if (commandTextarea) commandTextarea.focus();
    }
</script>

<div class="app-container">
    <!-- ===== TERMINAL PANEL ===== -->
    <div
        class="terminal-wrapper"
        class:drag-over={dragOver}
        on:dragover={handleDragOver}
        on:dragleave={handleDragLeave}
        on:drop={handleDrop}
    >
        <!-- Search bar overlay -->
        <SearchBar {searchAddon} />

        <!-- History panel overlay -->
        <HistoryPanel on:select={handleHistorySelect} />

        <!-- TABS BAR -->
        <div class="tabs-bar">
            {#each tabs as t (t.id)}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div class="tab" class:active={t.id === activeTabId} on:click={() => switchTab(t.id)}>
                    <span class="tab-title">{t.name}</span>
                    <button class="tab-close" on:click={(e) => closeTab(t.id, e)}>✕</button>
                </div>
            {/each}
            <button class="tab-new" on:click={() => createNewTab()}>+</button>
            <div class="tab-spacer"></div>
            <!-- Explorer Toggle Button -->
            <button class="sidebar-toggle" class:active={explorerVisible} on:click={() => explorerVisible = !explorerVisible} title="Toggle Explorer">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
                    <line x1="15" y1="3" x2="15" y2="21"></line>
                </svg>
            </button>
        </div>

        <!-- TERMINAL CONTAINERS -->
        <div class="terminals-container">
            {#each tabs as t (t.id)}
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div 
                    class="terminal-instance" 
                    class:active={t.id === activeTabId} 
                    bind:this={t.containerRef}
                    on:contextmenu={(e) => handleTerminalContext(e)}
                    on:click={() => t.term.focus()}
                ></div>
            {/each}
        </div>

        <div class="command-bar" style="position: relative;">
            <Autocomplete
                bind:this={autocompleteRef}
                partial={commandInput}
                {cwd}
                bind:visible={autocompleteVisible}
                on:select={handleAutocompleteSelect}
            />
            <span class="prompt-label">&#10095;</span>
            <textarea
                id="command-input"
                bind:this={commandTextarea}
                bind:value={commandInput}
                on:keydown={handleKeydown}
                placeholder="Type a command and press Enter..."
                autocomplete="off"
                spellcheck="false"
            ></textarea>
            <button id="run-btn" on:click={runCommand}>Run</button>
        </div>
    </div>

    <!-- ===== RESIZER ===== -->
    {#if explorerVisible}
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
        class="resizer"
        class:active={isResizing}
        on:mousedown={startResize}
    ></div>
    {/if}

    <!-- ===== FILE EXPLORER PANEL ===== -->
    {#if explorerVisible}
    <div class="explorer-panel" style="width: {explorerWidth}px;">
        <!-- Navigation bar: new folder / back / forward / up / pin -->
        <div class="explorer-nav">
            <button
                class="nav-btn nav-btn-icon"
                on:click={startCreateFolder}
                title="New Folder"
            >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path d="M1 4V13C1 13.55 1.45 14 2 14H14C14.55 14 15 13.55 15 13V6C15 5.45 14.55 5 14 5H8L6.5 3H2C1.45 3 1 3.45 1 4Z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
                    <path d="M8 8V12M6 10H10" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                </svg>
            </button>

            <button
                class="nav-btn"
                on:click={goBack}
                disabled={navBack.length === 0}
                title="Back"
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M10 3L5 8L10 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>
            <button
                class="nav-btn"
                on:click={goForward}
                disabled={navForward.length === 0}
                title="Forward"
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M6 3L11 8L6 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>
            <button
                class="nav-btn"
                on:click={goUp}
                title="Go up one level"
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M3 10L8 5L13 10" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>

            <div class="nav-spacer"></div>

            <button
                class="nav-btn nav-btn-icon"
                on:click={startEditPath}
                title="Edit path"
            >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path d="M11.5 1.5L14.5 4.5L5 14H2V11L11.5 1.5Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>
            <button
                class="nav-btn nav-btn-icon"
                on:click={copyPath}
                title="Copy full path"
            >
                {#if copied}
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                        <path d="M3 8.5L6.5 12L13 4" stroke="var(--dot-green, #27c93f)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                    </svg>
                {:else}
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                        <rect x="5" y="5" width="9" height="9" rx="1.5" stroke="currentColor" stroke-width="1.5"/>
                        <path d="M3 11V3C3 2.45 3.45 2 4 2H12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                    </svg>
                {/if}
            </button>
            <button
                class="nav-btn nav-pin-btn"
                class:pinned
                on:click={() => (pinned = !pinned)}
                title={pinned ? "Unpin — explorer follows terminal" : "Pin — explorer stays independent"}
            >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path d="M9.5 1.5L14.5 6.5L10 11L8 13L3 8L5 6L9.5 1.5Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill={pinned ? "currentColor" : "none"}/>
                    <path d="M1 15L4.5 11.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                </svg>
            </button>
        </div>

        <!-- Breadcrumb path bar -->
        <div class="explorer-cwd" title={cwd}>
            {#if cwdEditing}
                <input
                    class="cwd-edit-input"
                    bind:value={cwdEditPath}
                    on:keydown={commitPathEdit}
                    on:blur={cancelPathEdit}
                    autofocus
                />
            {:else}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div class="breadcrumb-bar" on:dblclick={copyBreadcrumbPath}>
                    {#each breadcrumbs as crumb, i}
                        {#if i > 0}
                            <span class="breadcrumb-sep">&#10095;</span>
                        {/if}
                        <!-- svelte-ignore a11y-click-events-have-key-events -->
                        <!-- svelte-ignore a11y-no-static-element-interactions -->
                        <span
                            class="breadcrumb-item"
                            class:breadcrumb-active={i === breadcrumbs.length - 1}
                            on:click={() => navigateToBreadcrumb(crumb.path)}
                            title={crumb.path}
                        >{crumb.label}</span>
                    {/each}
                    {#if breadcrumbs.length === 0}
                        <span class="breadcrumb-item breadcrumb-active">Loading...</span>
                    {/if}
                </div>
            {/if}
        </div>

        <!-- File filter -->
        <div class="explorer-filter">
            <input
                class="explorer-filter-input"
                bind:value={filterText}
                placeholder="Filter files..."
                spellcheck="false"
            />
        </div>

        <div class="explorer-tree">
            {#if creatingFolder}
                <div class="new-folder-row">
                    <span class="new-folder-icon">&#128193;</span>
                    <!-- svelte-ignore a11y-autofocus -->
                    <input
                        class="new-folder-input"
                        bind:value={newFolderName}
                        on:keydown={commitCreateFolder}
                        on:blur={cancelCreateFolder}
                        placeholder="Folder name..."
                        autofocus
                    />
                </div>
            {/if}
            {#if explorerLoading}
                <div class="explorer-loading">Loading...</div>
            {:else if filteredEntries.length === 0}
                <div class="explorer-empty">
                    {filterText ? "No matches" : "No files found"}
                </div>
            {:else}
                {#each filteredEntries as entry (entry.path)}
                    <TreeNode
                        name={entry.name}
                        path={entry.path}
                        isDir={entry.isDir}
                        depth={0}
                        {renamingPath}
                        {renameNewName}
                        {expandToPath}
                        on:filecontext={handleFileContext}
                        on:rename={(e) => confirmRename(e.detail)}
                        on:cancelrename={cancelRename}
                        on:renameinput={(e) => (renameNewName = e.detail)}
                    />
                {/each}
            {/if}
        </div>
    </div>
    {/if}
</div>

<!-- Global overlays -->
<ContextMenu
    bind:visible={ctxMenuVisible}
    x={ctxMenuX}
    y={ctxMenuY}
    items={ctxMenuItems}
/>
<Toast bind:this={toastRef} />
