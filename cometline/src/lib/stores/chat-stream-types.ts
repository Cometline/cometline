import type { ChatItem } from '#lib/types.js';

export type StreamCtx = {
	assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
	reasoning: { current: { text: string; pending: boolean } | null };
	/** Carried across stream events the same way reducer state does for beat boundaries. */
	needsTextSeparator: boolean;
};

export interface SessionStream {
	run: number;
	abort: AbortController;
	ctx: StreamCtx;
}
