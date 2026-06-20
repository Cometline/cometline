import type { ChatItem, StreamEvent } from '../types';
import {
	anyReasoningPending,
	getReasoningSegments,
	reasoningTextLength
} from '../conversation/reasoning';

type DebugGlobal = typeof globalThis & { __cometlineDebugChat?: boolean };

const KEY = 'cometline.debugChat';

function preview(text: string | undefined, max = 100) {
	if (!text) return '';
	return text.length > max ? `${text.slice(0, max)}...` : text;
}

export function chatDebugEnabled() {
	const globalDebug = (globalThis as DebugGlobal).__cometlineDebugChat;
	if (globalDebug) return true;
	try {
		return typeof localStorage !== 'undefined' && localStorage.getItem(KEY) === 'true';
	} catch {
		return false;
	}
}

export function chatDebug(scope: string, payload: unknown) {
	if (!chatDebugEnabled()) return;
	console.debug(`[cometline:chat:${scope}]`, payload);
}

export function summarizeStreamEvent(event: StreamEvent) {
	if (event.type === 'text_delta') {
		return {
			type: event.type,
			deltaLength: event.delta.length,
			deltaPreview: preview(event.delta)
		};
	}
	if (event.type === 'reasoning_delta') {
		return {
			type: event.type,
			textLength: event.text.length,
			textPreview: preview(event.text)
		};
	}
	if (event.type === 'tool_call') {
		return { type: event.type, id: event.id, tool: event.tool, input: event.input };
	}
	if (event.type === 'tool_result') {
		return {
			type: event.type,
			id: event.id,
			tool: event.tool,
			outputLength: event.output.length,
			outputPreview: preview(event.output),
			error: event.error
		};
	}
	if (event.type === 'error') {
		return { type: event.type, code: event.code, message: event.message };
	}
	if (event.type === 'step_finish') {
		return { type: event.type, usage: event.usage };
	}
	return { type: event.type };
}

export function summarizeChatItem(item: ChatItem) {
	if (item.type === 'assistant') {
		return {
			id: item.id,
			type: item.type,
			textLength: item.text.length,
			textPreview: preview(item.text),
			pending: item.pending,
			reasoningLength: reasoningTextLength(item),
			reasoningPreview: preview(
				getReasoningSegments(item.reasoning)
					.map((segment) => segment.text)
					.join('\n\n')
			),
			reasoningPending: anyReasoningPending(item)
		};
	}
	if (item.type === 'user') {
		return {
			id: item.id,
			type: item.type,
			textLength: item.text.length,
			textPreview: preview(item.text),
			reveal: item.reveal
		};
	}
	if (item.type === 'tool') {
		return {
			id: item.id,
			type: item.type,
			toolId: item.toolId,
			toolName: item.toolName,
			pending: item.pending,
			outputLength: item.output?.length ?? 0,
			error: item.error,
			durationMs: item.durationMs
		};
	}
	return item;
}

export function summarizeChatItems(items: ChatItem[]) {
	return items.map(summarizeChatItem);
}
