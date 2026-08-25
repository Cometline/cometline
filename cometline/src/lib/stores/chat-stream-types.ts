import type { ChatItem } from '$lib/types';

export type StreamCtx = {
	assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
	reasoning: { current: { text: string; pending: boolean } | null };
};

export interface SessionStream {
	run: number;
	abort: AbortController;
	ctx: StreamCtx;
}
