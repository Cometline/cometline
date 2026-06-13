/**
 * start-chat action
 *
 * Encapsulates the decision tree for submitting a chat turn:
 * - First turn (empty conversation) runs an optional flight pre-step and sends
 *   the message without adding a duplicate user item.
 * - Subsequent turns send normally.
 * - The session is refreshed after every send so the title can update.
 */

export interface StartChatAdapter {
	readonly sessionId: string;
	readonly hasVisibleConversation: boolean;

	/** Stream the message to the backend. */
	send(text: string, opts?: { skipUser?: boolean }): Promise<void>;

	/** Optional pre-step for the first-turn animation. */
	onFirstTurnStart?(text: string): Promise<void>;

	/** Optional hook called after a first-turn send completes. */
	onFirstTurnComplete?(): void;

	/** Refresh session metadata (e.g. title) after the turn. */
	refreshSession(): Promise<void>;
}

export async function startChat(adapter: StartChatAdapter, text: string): Promise<void> {
	const firstTurn = !adapter.hasVisibleConversation;

	if (firstTurn && adapter.onFirstTurnStart) {
		await adapter.onFirstTurnStart(text);
	}

	await adapter.send(text, { skipUser: firstTurn });

	if (firstTurn && adapter.onFirstTurnComplete) {
		adapter.onFirstTurnComplete();
	}

	await adapter.refreshSession();
}
