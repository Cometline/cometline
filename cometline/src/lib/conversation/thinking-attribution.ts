import type { ChatItem } from '$lib/types';
import { getReasoningSegments } from './reasoning';

export type InjectedMemory = Extract<ChatItem, { type: 'memory' }>['memories'][number];
export type ToolChatItem = Extract<ChatItem, { type: 'tool' }>;
export type SubagentChatItem = Extract<ChatItem, { type: 'subagent' }>;
type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

export type ThinkingBlock = {
	reasoning?: { text: string; pending?: boolean };
	tools: ToolChatItem[];
	subagents: SubagentChatItem[];
	memories: InjectedMemory[];
};

export type TimelineEntry =
	| {
			kind: 'reasoning';
			segmentIndex: number;
			text: string;
			pending?: boolean;
			memories?: InjectedMemory[];
	  }
	| { kind: 'tool'; tool: ToolChatItem }
	| { kind: 'subagent'; subagent: SubagentChatItem };

export type ThinkingAttribution = {
	map: Map<string, ThinkingBlock>;
	toolIdsInBuffer: Set<string>;
	subagentIdsInBuffer: Set<string>;
	memoryIdsInBuffer: Set<string>;
};

/** Attribute memory/tool rows to the assistant in the same user turn (full transcript scan). */
export function buildThinkingAttribution(items: readonly ChatItem[]): ThinkingAttribution {
	const map = new Map<string, ThinkingBlock>();
	const toolIdsInBuffer = new Set<string>();
	const subagentIdsInBuffer = new Set<string>();
	const memoryIdsInBuffer = new Set<string>();
	let currentAssistantId: string | null = null;
	let pendingMemories: InjectedMemory[] = [];

	for (let index = 0; index < items.length; index++) {
		const item = items[index];
		if (item.type === 'user' || item.type === 'status' || item.type === 'error') {
			currentAssistantId = null;
			pendingMemories = [];
			continue;
		}
		if (item.type === 'memory') {
			memoryIdsInBuffer.add(item.id);
			if (currentAssistantId) {
				const block = map.get(currentAssistantId);
				if (block) {
					block.memories = item.memories;
					continue;
				}
			}
			pendingMemories = item.memories;
			continue;
		}
		if (item.type === 'assistant') {
			currentAssistantId = item.id;
			const segments = getReasoningSegments(item.reasoning);
			const firstSegment = segments[0];
			const existing = map.get(item.id);
			if (!existing) {
				map.set(item.id, {
					reasoning: firstSegment
						? { text: firstSegment.text, pending: firstSegment.pending }
						: undefined,
					tools: [],
					subagents: [],
					memories: pendingMemories
				});
			} else {
				if (firstSegment && !existing.reasoning) {
					existing.reasoning = {
						text: firstSegment.text,
						pending: firstSegment.pending
					};
				}
				if (pendingMemories.length > 0) {
					existing.memories = pendingMemories;
				}
			}
			pendingMemories = [];
		} else if (item.type === 'tool' && currentAssistantId) {
			const block = map.get(currentAssistantId);
			if (block) {
				block.tools.push(item);
				toolIdsInBuffer.add(item.id);
			}
		} else if (item.type === 'subagent' && currentAssistantId) {
			const block = map.get(currentAssistantId);
			if (block) {
				block.subagents.push(item);
				subagentIdsInBuffer.add(item.id);
			}
		}
	}

	return { map, toolIdsInBuffer, subagentIdsInBuffer, memoryIdsInBuffer };
}

/** Build chronological thinking/tool entries for one assistant turn. */
export function buildAssistantTimeline(
	assistantId: string,
	items: readonly ChatItem[],
	attribution?: ThinkingAttribution
): TimelineEntry[] {
	const attr = attribution ?? buildThinkingAttribution(items);
	const assistant = items.find(
		(item): item is AssistantItem => item.type === 'assistant' && item.id === assistantId
	);
	if (!assistant) return [];

	const block = attr.map.get(assistantId);
	const tools = block?.tools ?? [];
	const subagents = block?.subagents ?? [];
	const memories = block?.memories ?? [];
	const segments = getReasoningSegments(assistant.reasoning);
	const timeline: TimelineEntry[] = [];

	if (segments.length === 0) {
		for (const tool of tools) {
			timeline.push({ kind: 'tool', tool });
		}
		for (const subagent of subagents) {
			timeline.push({ kind: 'subagent', subagent });
		}
		return timeline;
	}

	for (let i = 0; i < segments.length; i++) {
		const segment = segments[i];
		timeline.push({
			kind: 'reasoning',
			segmentIndex: i,
			text: segment.text,
			pending: segment.pending,
			memories: i === 0 && memories.length > 0 ? memories : undefined
		});
		for (const tool of tools) {
			const placement = tool.afterSegment ?? segments.length - 1;
			if (placement === i) {
				timeline.push({ kind: 'tool', tool });
			}
		}
	}

	const placed = new Set(
		tools.filter((tool) => tool.afterSegment !== undefined).map((tool) => tool.id)
	);
	for (const tool of tools) {
		if (!placed.has(tool.id)) {
			timeline.push({ kind: 'tool', tool });
		}
	}

	for (const subagent of subagents) {
		timeline.push({ kind: 'subagent', subagent });
	}

	return timeline;
}
