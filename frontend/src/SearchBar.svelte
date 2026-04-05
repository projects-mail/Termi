<script>
    import { searchOpen } from "./stores.js";

    export let searchAddon = null;

    let query = "";
    let inputEl;
    let matchCount = "";

    $: if ($searchOpen && inputEl) {
        setTimeout(() => inputEl?.focus(), 50);
    }

    function findNext() {
        if (searchAddon && query) {
            searchAddon.findNext(query);
        }
    }

    function findPrev() {
        if (searchAddon && query) {
            searchAddon.findPrevious(query);
        }
    }

    function close() {
        searchOpen.set(false);
        if (searchAddon) searchAddon.clearDecorations();
        query = "";
    }

    function handleKeydown(e) {
        if (e.key === "Escape") {
            close();
        } else if (e.key === "Enter") {
            e.preventDefault();
            if (e.shiftKey) {
                findPrev();
            } else {
                findNext();
            }
        }
    }

    function onInput() {
        if (searchAddon && query) {
            searchAddon.findNext(query);
        }
    }
</script>

{#if $searchOpen}
    <div class="search-bar">
        <input
            bind:this={inputEl}
            bind:value={query}
            on:keydown={handleKeydown}
            on:input={onInput}
            placeholder="Search..."
            spellcheck="false"
            class="search-input"
        />
        <button class="search-btn" on:click={findPrev} title="Previous (Shift+Enter)">&#9650;</button>
        <button class="search-btn" on:click={findNext} title="Next (Enter)">&#9660;</button>
        <button class="search-btn search-close" on:click={close} title="Close (Escape)">✕</button>
    </div>
{/if}

<style>
    .search-bar {
        position: absolute;
        top: 48px;
        right: 12px;
        display: flex;
        align-items: center;
        gap: 4px;
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 8px;
        padding: 6px 10px;
        box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
        z-index: 100;
    }

    .search-input {
        background: var(--input-bg, #0f0f1a);
        color: var(--text-primary, #e0e0e0);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 4px;
        padding: 5px 10px;
        font-family: Consolas, monospace;
        font-size: 13px;
        outline: none;
        width: 200px;
        transition: border-color 0.2s;
    }

    .search-input:focus {
        border-color: var(--accent, #00d4ff);
    }

    .search-btn {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        font-size: 12px;
        cursor: pointer;
        padding: 4px 6px;
        border-radius: 4px;
        transition: background 0.15s, color 0.15s;
    }

    .search-btn:hover {
        background: rgba(0, 212, 255, 0.1);
        color: var(--accent, #00d4ff);
    }

    .search-close {
        font-size: 14px;
        margin-left: 2px;
    }
</style>
