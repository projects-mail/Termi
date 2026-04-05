<script>
    import { GetCompletions } from "../wailsjs/go/main/App.js";
    import { createEventDispatcher } from "svelte";

    export let partial = "";
    export let cwd = "";
    export let visible = false;

    const dispatch = createEventDispatcher();

    let suggestions = [];
    let selectedIndex = -1;
    let debounceTimer;

    $: {
        // Debounced fetch when partial changes
        clearTimeout(debounceTimer);
        if (partial && partial.trim().length > 0) {
            debounceTimer = setTimeout(() => fetchCompletions(), 150);
        } else {
            suggestions = [];
            visible = false;
        }
    }

    async function fetchCompletions() {
        // Get the last "word" for matching
        const words = partial.trim().split(/\s+/);
        const lastWord = words[words.length - 1] || "";
        if (!lastWord) {
            suggestions = [];
            visible = false;
            return;
        }

        const results = await GetCompletions(partial.trim(), cwd);
        suggestions = results || [];
        selectedIndex = -1;
        visible = suggestions.length > 0;
    }

    export function handleKeydown(e) {
        if (!visible) return false;

        if (e.key === "ArrowDown") {
            e.preventDefault();
            selectedIndex = (selectedIndex + 1) % suggestions.length;
            return true;
        }
        if (e.key === "ArrowUp") {
            e.preventDefault();
            selectedIndex =
                selectedIndex <= 0
                    ? suggestions.length - 1
                    : selectedIndex - 1;
            return true;
        }
        if (e.key === "Tab" || (e.key === "Enter" && selectedIndex >= 0)) {
            e.preventDefault();
            selectItem(suggestions[selectedIndex >= 0 ? selectedIndex : 0]);
            return true;
        }
        if (e.key === "Escape") {
            visible = false;
            return true;
        }
        return false;
    }

    function selectItem(item) {
        dispatch("select", item);
        visible = false;
        suggestions = [];
        selectedIndex = -1;
    }

    export function hide() {
        visible = false;
    }
</script>

{#if visible && suggestions.length > 0}
    <div class="autocomplete-dropdown">
        {#each suggestions as item, i}
            <!-- svelte-ignore a11y-click-events-have-key-events -->
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div
                class="autocomplete-item"
                class:selected={i === selectedIndex}
                on:mousedown|preventDefault={() => selectItem(item)}
                on:mouseenter={() => (selectedIndex = i)}
            >
                {item}
            </div>
        {/each}
    </div>
{/if}

<style>
    .autocomplete-dropdown {
        position: absolute;
        bottom: 100%;
        left: 32px;
        right: 80px;
        max-height: 200px;
        overflow-y: auto;
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 8px;
        margin-bottom: 4px;
        box-shadow: 0 -8px 32px rgba(0, 0, 0, 0.4);
        z-index: 200;
        scrollbar-width: thin;
        scrollbar-color: var(--border-color, #2a2a4a) transparent;
    }

    .autocomplete-item {
        padding: 7px 14px;
        font-family: Consolas, monospace;
        font-size: 13px;
        color: var(--text-primary, #e0e0e0);
        cursor: pointer;
        transition: background 0.1s;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .autocomplete-item:hover,
    .autocomplete-item.selected {
        background: rgba(0, 212, 255, 0.1);
    }

    .autocomplete-item.selected {
        color: var(--accent, #00d4ff);
    }
</style>
