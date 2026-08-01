import { SvelteMap } from 'svelte/reactivity';

/**
 * Per-session reasoning effort toggle state.
 *
 * The composer Ctrl+T shortcut writes here instead of persisting settings,
 * so toggling is instant (no IPC / settings-file round trip). The value is
 * attached to each chat turn request and lives for the app session only.
 */
const bySession = new SvelteMap<string, string>();

export function getReasoningEffort(sessionId: string): string {
	return bySession.get(sessionId) ?? '';
}

export function setReasoningEffort(sessionId: string, effort: string): void {
	if (effort) {
		bySession.set(sessionId, effort);
	} else {
		bySession.delete(sessionId);
	}
}
