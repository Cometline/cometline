/**
 * start-chat action
 *
 * Encapsulates the decision tree for submitting a chat turn:
 * - Every turn can run an optional composer→thread user-bubble flight (staging +
 *   skipUser on send).
 * - First turn additionally runs layout/avatar choreography via the same hook.
 * - The session is refreshed after every send so the title can update.
 */

export interface StartChatAdapter {
	readonly sessionId: string;
	readonly hasVisibleConversation: boolean;

	/** Stream the message to the backend. */
	send(text: string, opts?: { skipUser?: boolean }): Promise<void>;

	/**
	 * Stage the user bubble and run the composer→thread flight before send.
	 * When provided, send uses skipUser because the item is already staged.
	 */
	onUserMessageFlight?(text: string, ctx: { firstTurn: boolean }): void | Promise<void>;

	/** Optional hook called after a first-turn send completes. */
	onFirstTurnComplete?(): void;

	/** Refresh session metadata (e.g. title) after the turn. */
	refreshSession(): Promise<void>;
}

export async function startChat(adapter: StartChatAdapter, text: string): Promise<void> {
	const firstTurn = !adapter.hasVisibleConversation;
	const usesFlight = Boolean(adapter.onUserMessageFlight);

	if (usesFlight) {
		await adapter.onUserMessageFlight!(text, { firstTurn });
	}

	await adapter.send(text, { skipUser: usesFlight ? true : firstTurn });

	if (firstTurn && adapter.onFirstTurnComplete) {
		adapter.onFirstTurnComplete();
	}

	await adapter.refreshSession();
}
