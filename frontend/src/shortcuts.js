/**
 * Register global keyboard shortcuts.
 * Uses capture phase so shortcuts fire before xterm.js consumes keys.
 *
 * @param {Object} handlers
 * @param {Function} handlers.clearTerminal
 * @param {Function} handlers.copy
 * @param {Function} handlers.paste
 * @param {Function} handlers.toggleHistory
 * @param {Function} handlers.toggleSearch
 */
export function registerShortcuts(handlers) {
    window.addEventListener(
        "keydown",
        (e) => {
            const ctrl = e.ctrlKey || e.metaKey;

            // Ctrl+L — Clear terminal
            if (ctrl && !e.shiftKey && e.key.toLowerCase() === "l") {
                e.preventDefault();
                handlers.clearTerminal();
                return;
            }

            // Ctrl+Shift+C — Copy selection
            if (ctrl && e.shiftKey && e.key.toLowerCase() === "c") {
                e.preventDefault();
                handlers.copy();
                return;
            }

            // Ctrl+Shift+V — Paste
            if (ctrl && e.shiftKey && e.key.toLowerCase() === "v") {
                e.preventDefault();
                handlers.paste();
                return;
            }

            // Ctrl+K — Toggle history panel
            if (ctrl && !e.shiftKey && e.key.toLowerCase() === "k") {
                e.preventDefault();
                handlers.toggleHistory();
                return;
            }

            // Ctrl+F — Toggle search
            if (ctrl && !e.shiftKey && e.key.toLowerCase() === "f") {
                e.preventDefault();
                handlers.toggleSearch();
                return;
            }
        },
        true, // capture phase — fires before xterm.js
    );
}
