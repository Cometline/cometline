import type { ChatItem } from '$lib/types';
import type { StreamEvent } from '$lib/types';

export type StreamCtx = {
	assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
	reasoning: { current: { text: string; pending: boolean } | null };
};

export interface SessionStream {
	run: number;
	abort: AbortController;
	pendingBatchEvents: StreamEvent[];
	batchFrame: number;
	ctx: StreamCtx;
}

export const BATCHABLE_STREAM_EVENTS = new Set([
	'text_delta',
	'reasoning_delta',
	'reasoning_start'
]);
