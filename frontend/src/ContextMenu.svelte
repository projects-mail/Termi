<script>
    import { onMount, onDestroy } from "svelte";

    export let x = 0;
    export let y = 0;
    export let items = [];
    export let visible = false;

    let menuEl;

    // Adjust position if menu would overflow viewport
    $: if (visible && menuEl) {
        requestAnimationFrame(() => {
            if (!menuEl) return;
            const rect = menuEl.getBoundingClientRect();
            if (rect.right > window.innerWidth) {
                menuEl.style.left = window.innerWidth - rect.width - 8 + "px";
            }
            if (rect.bottom > window.innerHeight) {
                menuEl.style.top = window.innerHeight - rect.height - 8 + "px";
            }
        });
    }

    function handleClick(item) {
        if (item.action) item.action();
        visible = false;
    }

    function handleOutsideClick(e) {
        if (visible && menuEl && !menuEl.contains(e.target)) {
            visible = false;
        }
    }

    function handleKeydown(e) {
        if (e.key === "Escape") {
            visible = false;
        }
    }
</script>

<svelte:window
    on:mousedown={handleOutsideClick}
    on:keydown={handleKeydown}
    on:scroll={() => (visible = false)}
/>

{#if visible}
    <div
        class="context-menu"
        bind:this={menuEl}
        style="left: {x}px; top: {y}px"
    >
        {#each items as item}
            {#if item.divider}
                <div class="context-divider"></div>
            {:else}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div
                    class="context-item"
                    on:click={() => handleClick(item)}
                >
                    <span class="context-label">{item.label}</span>
                    {#if item.shortcut}
                        <span class="context-shortcut">{item.shortcut}</span>
                    {/if}
                </div>
            {/if}
        {/each}
    </div>
{/if}

<style>
    .context-menu {
        position: fixed;
        z-index: 2000;
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 8px;
        padding: 4px 0;
        min-width: 180px;
        box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
        animation: ctxFadeIn 0.12s ease-out;
    }

    @keyframes ctxFadeIn {
        from {
            opacity: 0;
            transform: scale(0.95);
        }
        to {
            opacity: 1;
            transform: scale(1);
        }
    }

    .context-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 7px 14px;
        cursor: pointer;
        transition: background 0.1s;
        user-select: none;
    }

    .context-item:hover {
        background: rgba(0, 212, 255, 0.1);
    }

    .context-label {
        font-size: 13px;
        color: var(--text-primary, #e0e0e0);
    }

    .context-shortcut {
        font-size: 11px;
        color: var(--text-muted, #888);
        margin-left: 24px;
    }

    .context-divider {
        height: 1px;
        background: var(--border-color, #2a2a4a);
        margin: 4px 8px;
    }
</style>
