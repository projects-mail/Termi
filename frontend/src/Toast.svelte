<script>
    import { onMount, onDestroy } from "svelte";
    import { EventsOn } from "../wailsjs/runtime/runtime.js";

    let toasts = [];
    let nextId = 0;

    function addToast(message) {
        const id = nextId++;
        toasts = [...toasts, { id, message, fading: false }];

        // Start fade-out after 4.5s, remove at 5s
        setTimeout(() => {
            toasts = toasts.map((t) =>
                t.id === id ? { ...t, fading: true } : t,
            );
        }, 4500);

        setTimeout(() => {
            toasts = toasts.filter((t) => t.id !== id);
        }, 5000);
    }

    onMount(() => {
        EventsOn("command-complete", (elapsed) => {
            const secs = Math.round(elapsed * 10) / 10;
            addToast(`Command finished (${secs}s)`);
        });
    });
</script>

<div class="toast-container">
    {#each toasts as toast (toast.id)}
        <div class="toast" class:fade-out={toast.fading}>
            <span class="toast-icon">&#9889;</span>
            {toast.message}
        </div>
    {/each}
</div>

<style>
    .toast-icon {
        margin-right: 6px;
    }
</style>
