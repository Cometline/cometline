import type { TerminalSnapshot } from '$lib/types';
import { shellStore } from '$lib/stores/shell.svelte';

const MAX_OUTPUT_CHARS = 2_000_000;

function appendOutput(output: string, data: string) {
	const next = output + data;
	return next.length > MAX_OUTPUT_CHARS ? next.slice(-MAX_OUTPUT_CHARS) : next;
}

function createTerminalStore() {
	let terminals = $state<Record<string, TerminalSnapshot>>({});
	let initialized = false;
	const listeners = new Map<string, Set<(data: string) => void>>();

	function setTerminal(snapshot: TerminalSnapshot) {
		terminals = { ...terminals, [snapshot.sessionId]: snapshot };
	}

	function clearTerminal(sessionId: string) {
		if (!(sessionId in terminals)) return;
		const next = { ...terminals };
		delete next[sessionId];
		terminals = next;
		listeners.delete(sessionId);
	}

	function notify(sessionId: string, data: string) {
		for (const listener of listeners.get(sessionId) ?? []) listener(data);
	}

	return {
		get sessionIds() {
			return Object.keys(terminals);
		},
		getSnapshot(sessionId: string) {
			return terminals[sessionId] ?? null;
		},
		hasTerminal(sessionId: string) {
			return sessionId in terminals;
		},
		isRunning(sessionId: string) {
			return terminals[sessionId]?.status === 'running';
		},
		async initialize() {
			if (initialized) return;
			initialized = true;
			const api = window.electronAPI;
			if (!api?.listTerminals) return;
			try {
				for (const snapshot of await api.listTerminals()) setTerminal(snapshot);
			} catch {
				// Terminal support is optional outside Electron and during startup failures.
			}
			api.onTerminalData?.(({ sessionId, data }) => {
				const current = terminals[sessionId];
				if (!current) return;
				setTerminal({ ...current, output: appendOutput(current.output, data) });
				notify(sessionId, data);
			});
			api.onTerminalExit?.((snapshot) => {
				clearTerminal(snapshot.sessionId);
				shellStore.closeTerminalPanelForSession(snapshot.sessionId);
			});
		},
		async start(sessionId: string, workspacePath: string, cols = 80, rows = 24) {
			const api = window.electronAPI;
			if (!api?.createTerminal)
				throw new Error('Terminal integration is only available in Cometline.');
			const snapshot = await api.createTerminal({ sessionId, workspacePath, cols, rows });
			setTerminal(snapshot);
			return snapshot;
		},
		async write(sessionId: string, data: string) {
			return (await window.electronAPI?.writeTerminal?.({ sessionId, data })) ?? false;
		},
		async resize(sessionId: string, cols: number, rows: number) {
			return (await window.electronAPI?.resizeTerminal?.({ sessionId, cols, rows })) ?? false;
		},
		async terminate(sessionId: string) {
			return (await window.electronAPI?.terminateTerminal?.(sessionId)) ?? false;
		},
		async remove(sessionId: string) {
			const removed = (await window.electronAPI?.removeTerminal?.(sessionId)) ?? false;
			if (removed) clearTerminal(sessionId);
			return removed;
		},
		subscribe(sessionId: string, listener: (data: string) => void) {
			const sessionListeners = listeners.get(sessionId) ?? new Set<(data: string) => void>();
			sessionListeners.add(listener);
			listeners.set(sessionId, sessionListeners);
			return () => {
				sessionListeners.delete(listener);
				if (sessionListeners.size === 0) listeners.delete(sessionId);
			};
		}
	};
}

export const terminalStore = createTerminalStore();
