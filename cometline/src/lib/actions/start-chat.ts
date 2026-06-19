/**
 * start-chat action
 *
 * Encapsulates the decision tree for submitting a chat turn:
 * - Every turn can run an optional composer→thread user-bubble flight (staging +
 *   skipUser on send).
 * - First turn additionally runs layout/avatar choreography via the same hook.
 * - The session is refreshed after every send so the title can update.
 */

import type { ImageAttachment } from '$lib/types';

export interface ChatTurnPayload {
	text: string;
	images?: ImageAttachment[];
	filePaths?: string[];
}

export interface StartChatAdapter {
	readonly sessionId: string;
	readonly hasVisibleConversation: boolean;

	/** Stream the message to the backend. */
	send(payload: ChatTurnPayload | string, opts?: { skipUser?: boolean }): Promise<void>;

	/**
	 * Stage the user bubble and run the composer→thread flight before send.
	 * When provided, send uses skipUser because the item is already staged.
	 */
	onUserMessageFlight?(payload: ChatTurnPayload | string, ctx: { firstTurn: boolean }): void | Promise<void>;

	/** Optional hook called after a first-turn send completes. */
	onFirstTurnComplete?(): void;

	/** Refresh session metadata (e.g. title) after the turn. */
	refreshSession(): Promise<void>;
}

export async function startChat(
	adapter: StartChatAdapter,
	payloadOrText: ChatTurnPayload | string
): Promise<void> {
	const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
	const firstTurn = !adapter.hasVisibleConversation;
	const usesFlight = Boolean(adapter.onUserMessageFlight);

	if (usesFlight) {
		await adapter.onUserMessageFlight!(payload.images?.length ? payload : payload.text, { firstTurn });
	}

	await adapter.send(payloadOrText, { skipUser: usesFlight ? true : firstTurn });

	if (firstTurn && adapter.onFirstTurnComplete) {
		adapter.onFirstTurnComplete();
	}

	void adapter.refreshSession();
}
