import { browser } from '$app/environment';
import {
	CometMindApiError,
	listAllSessions,
	runStorageRetention,
	type RunStorageRetentionResponse
} from '$lib/client/cometmind';
import type { CometMindStorageSettings } from '$lib/settings/schema';
import { activateAfterSessionDeleted } from '$lib/actions/activate-after-session-deleted';
import { connectionState } from '$lib/stores/runtime.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import type { Session } from '$lib/types';

const DEFAULT_CLEANUP_INTERVAL_MINUTES = 60;

function cleanupIntervalMs(storage: CometMindStorageSettings): number {
	const minutes = storage.cleanupIntervalMinutes || DEFAULT_CLEANUP_INTERVAL_MINUTES;
	return Math.max(1, Math.floor(minutes)) * 60_000;
}

export async function runStorageRetentionAndSyncSessions(): Promise<
	RunStorageRetentionResponse | undefined
> {
	if (connectionState.status !== 'ready') return undefined;
	try {
		const previous = sessionStore.sessions;
		const result = await runStorageRetention();
		const sessions = await listAllSessions();
		await syncSessionsAfterRetention(previous, sessions.sessions);
		return result;
	} catch (err) {
		if (err instanceof CometMindApiError && err.status === 503) {
			return undefined;
		}
		throw err;
	}
}

async function syncSessionsAfterRetention(previous: Session[], next: Session[]) {
	const nextIDs = new Set(next.map((session) => session.id));
	const removed = previous.filter((session) => !nextIDs.has(session.id));
	const activeId = sessionStore.current?.id ?? null;
	const activeWasRemoved = Boolean(activeId && removed.some((session) => session.id === activeId));

	for (const session of removed) {
		shellStore.clearWebPanelForSession(session.id);
		sessionStore.removeSession(session.id);
	}
	sessionStore.setSessions(next);

	if (activeWasRemoved && activeId && browser) {
		await activateAfterSessionDeleted(activeId, previous);
	}
}

export function startStorageRetentionSync(getStorage: () => CometMindStorageSettings): () => void {
	let stopped = false;
	let timer: ReturnType<typeof setTimeout> | null = null;
	let inFlight = false;

	function scheduleNext() {
		if (stopped) return;
		if (timer) clearTimeout(timer);
		timer = setTimeout(tick, cleanupIntervalMs(getStorage()));
	}

	async function tick() {
		if (stopped) return;
		if (!inFlight && connectionState.status === 'ready') {
			inFlight = true;
			try {
				await runStorageRetentionAndSyncSessions();
			} catch {
				// The connection poller and settings UI surface runtime health/errors.
			} finally {
				inFlight = false;
			}
		}
		scheduleNext();
	}

	scheduleNext();
	return () => {
		stopped = true;
		if (timer) clearTimeout(timer);
	};
}
