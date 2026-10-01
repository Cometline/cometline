import type { ChatItem } from '$lib/types';

export type ReasoningSegment = { text: string; pending?: boolean };

type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;
type AssistantReasoning = NonNullable<AssistantItem['reasoning']>;

/** Normalize legacy flat `reasoning.text` into segments. */
export function getReasoningSegments(
	reasoning: AssistantReasoning | undefined
): ReasoningSegment[] {
	if (!reasoning) return [];
	if (reasoning.segments?.length) return reasoning.segments;
	if (reasoning.text?.trim() || reasoning.pending) {
		return [{ text: reasoning.text ?? '', pending: reasoning.pending }];
	}
	return [];
}

export function hasReasoning(item: AssistantItem): boolean {
	return getReasoningSegments(item.reasoning).some(
		(segment) => segment.text.trim().length > 0 || segment.pending
	);
}

export function reasoningTextLength(item: AssistantItem): number {
	return getReasoningSegments(item.reasoning).reduce(
		(total, segment) => total + segment.text.length,
		0
	);
}

export function anyReasoningPending(item: AssistantItem): boolean {
	return getReasoningSegments(item.reasoning).some((segment) => segment.pending);
}

export function cloneReasoning(
	reasoning: AssistantReasoning | undefined
): AssistantReasoning | undefined {
	if (!reasoning) return undefined;
	const segments = getReasoningSegments(reasoning).map((segment) => ({ ...segment }));
	return { segments };
}
