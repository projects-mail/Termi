<script>
    import { createEventDispatcher } from "svelte";
    import { ListDirectory, OpenFileInEditor } from "../wailsjs/go/main/App.js";

    export let name = "";
    export let path = "";
    export let isDir = false;
    export let depth = 0;
    export let renamingPath = "";
    export let renameNewName = "";
    // When set, this folder (and its ancestors) should auto-expand to reveal this path
    export let expandToPath = "";

    const dispatch = createEventDispatcher();

    let open = false;
    let children = [];
    let loaded = false;
    let loading = false;

    async function loadChildren() {
        if (loaded) return;
        loading = true;
        try {
            children = await ListDirectory(path);
        } catch (e) {
            children = [];
        } finally {
            loaded = true;
            loading = false;
        }
    }

    // Auto-expand if this folder is an ancestor of expandToPath
    $: if (isDir && expandToPath && isAncestorOf(path, expandToPath)) {
        autoExpand();
    }

    function normalizePath(p) {
        return p.replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
    }

    function isAncestorOf(ancestor, descendant) {
        const a = normalizePath(ancestor);
        const d = normalizePath(descendant);
        if (a === d) return false;
        return d.startsWith(a + "/") || d.startsWith(a);
    }

    async function autoExpand() {
        if (!open) {
            open = true;
            await loadChildren();
        }
    }

    // Single click on folder: just expand/collapse. Files: do nothing.
    async function handleClick() {
        if (!isDir) return;
        open = !open;
        if (open && !loaded) {
            await loadChildren();
        }
    }

    function getFileIcon() {
        if (isDir) return open ? "\uD83D\uDCC2" : "\uD83D\uDCC1";
        const ext = name.split(".").pop().toLowerCase();
        const icons = {
            js: "\uD83D\uDFE8",
            ts: "\uD83D\uDD37",
            svelte: "\uD83D\uDFE0",
            go: "\uD83D\uDC39",
            json: "\uD83D\uDCCB",
            md: "\uD83D\uDCDD",
            css: "\uD83C\uDFA8",
            html: "\uD83C\uDF10",
            txt: "\uD83D\uDCC4",
            png: "\uD83D\uDDBC\uFE0F",
            jpg: "\uD83D\uDDBC\uFE0F",
            gif: "\uD83D\uDDBC\uFE0F",
            svg: "\uD83D\uDDBC\uFE0F",
            exe: "\u2699\uFE0F",
            ps1: "\uD83D\uDC99",
            sh: "\uD83D\uDC1A",
        };
        return icons[ext] || "\uD83D\uDCC4";
    }

    // Double-click: files open in editor
    function handleDblClick() {
        if (!isDir) {
            OpenFileInEditor(path);
        }
    }

    // Right-click: dispatch context menu event
    function handleContextMenu(e) {
        e.preventDefault();
        e.stopPropagation();
        dispatch("filecontext", {
            x: e.clientX,
            y: e.clientY,
            entry: { name, path, isDir },
        });
    }

    // Drag
    function handleDragStart(e) {
        e.dataTransfer.setData("text/plain", path);
        e.dataTransfer.effectAllowed = "copy";
    }

    // Rename
    function handleRenameKeydown(e) {
        if (e.key === "Enter") {
            e.preventDefault();
            dispatch("rename", { name, path, isDir });
        } else if (e.key === "Escape") {
            dispatch("cancelrename");
        }
    }

    function handleRenameInput(e) {
        dispatch("renameinput", e.target.value);
    }

    // Bubble events from child TreeNodes
    function bubbleFileContext(e) { dispatch("filecontext", e.detail); }
    function bubbleRename(e) { dispatch("rename", e.detail); }
    function bubbleCancelRename() { dispatch("cancelrename"); }
    function bubbleRenameInput(e) { dispatch("renameinput", e.detail); }

    // Check if this folder IS the expandToPath target (highlight it)
    $: isHighlighted = isDir && expandToPath && normalizePath(path) === normalizePath(expandToPath);
</script>

<div class="tree-item" style="padding-left: {depth * 14}px">
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
        class="item-row {isDir ? 'is-dir' : 'is-file'}"
        class:highlighted={isHighlighted}
        on:click={handleClick}
        on:dblclick={handleDblClick}
        on:contextmenu={handleContextMenu}
        draggable="true"
        on:dragstart={handleDragStart}
    >
        {#if isDir}
            <span class="caret">{open ? "\u25BE" : "\u25B8"}</span>
        {:else}
            <span class="caret-spacer"></span>
        {/if}
        <span class="item-icon">{getFileIcon()}</span>

        {#if renamingPath === path}
            <!-- svelte-ignore a11y-autofocus -->
            <input
                class="rename-input"
                value={renameNewName}
                on:input={handleRenameInput}
                on:keydown={handleRenameKeydown}
                on:blur={() => dispatch("cancelrename")}
                autofocus
                on:click|stopPropagation
            />
        {:else}
            <span class="item-name" title={path}>{name}</span>
        {/if}

        {#if loading}
            <span class="loader">&#8230;</span>
        {/if}
    </div>

    {#if open && children.length > 0}
        <div class="children">
            {#each children as child (child.path)}
                <svelte:self
                    name={child.name}
                    path={child.path}
                    isDir={child.isDir}
                    depth={depth + 1}
                    {renamingPath}
                    {renameNewName}
                    {expandToPath}
                    on:filecontext={bubbleFileContext}
                    on:rename={bubbleRename}
                    on:cancelrename={bubbleCancelRename}
                    on:renameinput={bubbleRenameInput}
                />
            {/each}
        </div>
    {/if}

    {#if open && loaded && children.length === 0 && !loading}
        <div
            class="empty-dir"
            style="padding-left: {(depth + 1) * 14 + 20}px"
        >
            <span class="text-muted">empty</span>
        </div>
    {/if}
</div>

<style>
    .tree-item {
        user-select: none;
    }

    .item-row {
        display: flex;
        align-items: center;
        gap: 5px;
        padding: 3px 8px 3px 0;
        border-radius: 4px;
        cursor: default;
        transition: background 0.12s;
        white-space: nowrap;
        overflow: hidden;
    }

    .item-row.is-dir {
        cursor: pointer;
    }

    .item-row:hover {
        background: rgba(0, 212, 255, 0.08);
    }

    /* Highlighted row = the folder we navigated from (auto-expanded target) */
    .item-row.highlighted {
        background: rgba(0, 212, 255, 0.12);
        border-left: 2px solid var(--accent, #00d4ff);
        padding-left: 0;
    }

    .caret {
        font-size: 10px;
        color: #888;
        width: 14px;
        height: 14px;
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        border-radius: 3px;
        cursor: pointer;
        transition: background 0.1s, color 0.1s;
    }

    .caret:hover {
        background: rgba(0, 212, 255, 0.15);
        color: var(--accent, #00d4ff);
    }

    .caret-spacer {
        width: 14px;
        flex-shrink: 0;
    }

    .item-icon {
        font-size: 13px;
        flex-shrink: 0;
    }

    .item-name {
        font-size: 12px;
        overflow: hidden;
        text-overflow: ellipsis;
        color: #c8c8d8;
        flex: 1;
    }

    .is-dir .item-name {
        color: #e0e0f0;
        font-weight: 500;
    }

    .loader {
        font-size: 10px;
        color: #555;
    }

    .empty-dir {
        padding: 2px 0;
    }

    .text-muted {
        font-size: 11px;
        color: #444;
        font-style: italic;
    }

    .rename-input {
        flex: 1;
        background: var(--input-bg, #0f0f1a);
        color: var(--accent, #00d4ff);
        border: 1px solid var(--accent, #00d4ff);
        border-radius: 3px;
        padding: 1px 5px;
        font-family: Consolas, monospace;
        font-size: 12px;
        outline: none;
    }
</style>
