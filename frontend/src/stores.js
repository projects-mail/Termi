import { writable } from "svelte/store";

// Panel visibility toggles
export const historyOpen = writable(false);
export const searchOpen = writable(false);

// Command history
export const commandHistory = writable([]);
