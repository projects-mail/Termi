<script>
    import { historyOpen, commandHistory } from "./stores.js";
    import {
        LoadCommandHistory,
        ClearCommandHistory,
    } from "../wailsjs/go/main/App.js";
    import { createEventDispatcher, onMount } from "svelte";

    const dispatch = createEventDispatcher();

    let filter = "";

    $: filtered = filter
        ? $commandHistory.filter((c) =>
              c.toLowerCase().includes(filter.toLowerCase()),
          )
        : $commandHistory;

    // Reload history from disk when panel opens
    $: if ($historyOpen) {
        loadHistory();
    }

    async function loadHistory() {
        const hist = await LoadCommandHistory();
        commandHistory.set(hist || []);
    }

    function selectCommand(cmd) {
        dispatch("select", cmd);
    }

    async function clearHistory() {
        await ClearCommandHistory();
        commandHistory.set([]);
    }

    function close() {
        historyOpen.set(false);
    }

    function handleKeydown(e) {
        if (e.key === "Escape") close();
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $historyOpen}
    <div class="history-panel">
        <div class="history-header">
            <span class="history-title">Command History</span>
            <div class="history-actions">
                <button class="history-clear" on:click={clearHistory} title="Clear history">Clear</button>
                <button class="history-close" on:click={close}>✕</button>
            </div>
        </div>

        <div class="history-filter">
            <input
                bind:value={filter}
                placeholder="Filter commands..."
                spellcheck="false"
                class="history-filter-input"
            />
        </div>

        <div class="history-list">
            {#if filtered.length === 0}
                <div class="history-empty">No commands found</div>
            {:else}
                {#each filtered as cmd, i}
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div
                        class="history-item"
                        on:click={() => selectCommand(cmd)}
                        title={cmd}
                    >
                        <span class="history-index">#{i + 1}</span>
                        <span class="history-cmd">{cmd}</span>
                    </div>
                {/each}
            {/if}
        </div>
    </div>
{/if}

<style>
    .history-panel {
        position: absolute;
        top: 48px;
        left: 12px;
        width: 360px;
        max-height: 60vh;
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 10px;
        box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
        z-index: 100;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .history-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 12px 14px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
        flex-shrink: 0;
    }

    .history-title {
        font-size: 12px;
        font-weight: 600;
        color: var(--text-muted, #888);
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .history-actions {
        display: flex;
        gap: 6px;
        align-items: center;
    }

    .history-clear {
        background: transparent;
        border: 1px solid var(--border-color, #2a2a4a);
        color: var(--text-muted, #888);
        font-size: 11px;
        padding: 3px 8px;
        border-radius: 4px;
        cursor: pointer;
        transition: background 0.15s, color 0.15s;
    }

    .history-clear:hover {
        background: rgba(240, 113, 120, 0.15);
        color: #f07178;
        border-color: #f07178;
    }

    .history-close {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        font-size: 14px;
        cursor: pointer;
        padding: 2px 6px;
        border-radius: 4px;
        transition: background 0.15s;
    }

    .history-close:hover {
        background: rgba(255, 255, 255, 0.08);
    }

    .history-filter {
        padding: 8px 12px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
        flex-shrink: 0;
    }

    .history-filter-input {
        width: 100%;
        background: var(--input-bg, #0f0f1a);
        color: var(--text-primary, #e0e0e0);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 5px;
        padding: 6px 10px;
        font-family: Consolas, monospace;
        font-size: 12px;
        outline: none;
        transition: border-color 0.2s;
    }

    .history-filter-input:focus {
        border-color: var(--accent, #00d4ff);
    }

    .history-list {
        flex: 1;
        overflow-y: auto;
        padding: 4px 0;
        scrollbar-width: thin;
        scrollbar-color: var(--border-color, #2a2a4a) transparent;
    }

    .history-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 7px 14px;
        cursor: pointer;
        transition: background 0.12s;
    }

    .history-item:hover {
        background: rgba(0, 212, 255, 0.08);
    }

    .history-index {
        font-size: 10px;
        color: var(--text-muted, #888);
        min-width: 24px;
    }

    .history-cmd {
        font-family: Consolas, monospace;
        font-size: 12px;
        color: var(--text-primary, #e0e0e0);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        flex: 1;
    }

    .history-empty {
        font-size: 12px;
        color: var(--text-muted, #888);
        padding: 20px;
        text-align: center;
        font-style: italic;
    }
</style>
