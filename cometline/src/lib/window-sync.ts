import { browser } from '$app/environment';
import type { ChatItem, Session } from '$lib/types';

type SyncPayload =
	| { type: 'session-upsert'; session: Session }
	| { type: 'session-remove'; sessionId: string }
	| { type: 'chat-items'; sessionId: string; items: ChatItem[] }
	| { type: 'chat-streaming'; sessionId: string; streaming: boolean };

type SyncMessage = SyncPayload & { source: string };

const source =
	typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
		? crypto.randomUUID()
		: `window-${Math.random().toString(36).slice(2)}`;

const listeners = new Set<(message: SyncMessage) => void>();

const channel = browser && 'BroadcastChannel' in globalThis
	? new BroadcastChannel('cometline-window-sync')
	: null;

if (channel) {
	channel.addEventListener('message', (event) => {
		const message = event.data as SyncMessage | undefined;
		if (!message || message.source === source) return;
		for (const listener of listeners) {
			listener(message);
		}
	});
}

export function publishWindowSync(message: SyncPayload) {
	if (!channel) return;
	const syncMessage = cloneForWindowSync({ source, ...message });
	if (!syncMessage) return;
	channel.postMessage(syncMessage);
}

function cloneForWindowSync(message: SyncMessage): SyncMessage | null {
	try {
		return JSON.parse(JSON.stringify(message)) as SyncMessage;
	} catch (err) {
		console.warn('Failed to publish window sync message', err);
		return null;
	}
}

export function subscribeWindowSync(listener: (message: SyncMessage) => void) {
	listeners.add(listener);
	return () => listeners.delete(listener);
}
