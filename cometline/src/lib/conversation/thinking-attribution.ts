import type { ChatItem } from '$lib/types';
import { getReasoningSegments } from './reasoning';

export type InjectedMemory = Extract<ChatItem, { type: 'memory' }>['memories'][number];
export type ToolChatItem = Extract<ChatItem, { type: 'tool' }>;
export type SubagentChatItem = Extract<ChatItem, { type: 'subagent' }>;
export type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

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
	  }
	| { kind: 'memory'; memories: InjectedMemory[] }
	| { kind: 'tool'; tool: ToolChatItem }
	| { kind: 'subagent'; subagent: SubagentChatItem };

export type ThinkingAttribution = {
	map: Map<string, ThinkingBlock>;
	toolIdsInBuffer: Set<string>;
	subagentIdsInBuffer: Set<string>;
	memoryIdsInBuffer: Set<string>;
};

/** Completed propose_job tools render as standalone rows (workspace picker must stay visible). */
export function isPinnedJobProposalTool(item: ToolChatItem): boolean {
	return item.toolName === 'propose_job' && !item.pending && !item.error;
}

function stopsPinnedJobScan(item: ChatItem): boolean {
	return (
		item.type === 'user' ||
		item.type === 'assistant' ||
		item.type === 'status' ||
		item.type === 'error'
	);
}

/** Pinned propose_job tools in the same assistant turn (survives reload item ordering). */
export function pinnedJobProposalsForAssistant(
	assistantId: string,
	items: readonly ChatItem[]
): ToolChatItem[] {
	const start = items.findIndex(
		(item): item is AssistantItem => item.type === 'assistant' && item.id === assistantId
	);
	if (start === -1) return [];

	const pinned: ToolChatItem[] = [];
	for (let i = start + 1; i < items.length; i++) {
		const item = items[i];
		if (stopsPinnedJobScan(item)) break;
		// Transcript reload inserts a standalone memory row between assistant and tools.
		if (item.type === 'memory') continue;
		if (item.type === 'tool' && isPinnedJobProposalTool(item)) {
			pinned.push(item);
			continue;
		}
		if (item.type === 'tool' || item.type === 'subagent') break;
	}
	return pinned;
}

/** Tool ids rendered inside assistant-stack instead of standalone tool rows. */
export function pinnedJobProposalToolIds(items: readonly ChatItem[]): Set<string> {
	const ids = new Set<string>();
	for (let i = 0; i < items.length; i++) {
		const item = items[i];
		if (item.type !== 'assistant') continue;
		for (const tool of pinnedJobProposalsForAssistant(item.id, items)) {
			ids.add(tool.id);
		}
	}
	return ids;
}

/** Attribute memory/tool rows to the assistant in the same user turn (full transcript scan). */
export function buildThinkingAttribution(items: readonly ChatItem[]): ThinkingAttribution {
	const map = new Map<string, ThinkingBlock>();
	const toolIdsInBuffer = new Set<string>();
	const subagentIdsInBuffer = new Set<string>();
	const memoryIdsInBuffer = new Set<string>();
	let currentAssistantId: string | null = null;
	let pendingMemories: InjectedMemory[] = [];
	let pendingMemoryId: string | null = null;

	for (let index = 0; index < items.length; index++) {
		const item = items[index];
		if (item.type === 'user' || item.type === 'status' || item.type === 'error') {
			currentAssistantId = null;
			pendingMemories = [];
			pendingMemoryId = null;
			continue;
		}
		if (item.type === 'memory') {
			// Attribute the memory to its assistant turn so it renders inside the
			// activity group/timeline. Only mark it buffered once it is actually
			// attached; an orphan memory (no assistant in the turn) falls back to
			// the standalone card.
			if (currentAssistantId) {
				const block = map.get(currentAssistantId);
				if (block) {
					block.memories = item.memories;
					memoryIdsInBuffer.add(item.id);
					continue;
				}
			}
			pendingMemories = item.memories;
			pendingMemoryId = item.id;
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
			if (pendingMemoryId && pendingMemories.length > 0) {
				memoryIdsInBuffer.add(pendingMemoryId);
			}
			pendingMemories = [];
			pendingMemoryId = null;
		} else if (item.type === 'tool' && currentAssistantId) {
			if (isPinnedJobProposalTool(item)) {
				continue;
			}
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

	// Memory is injected before any reasoning/tool activity, so it leads the timeline
	// as a first-class entry. This keeps the memory card visible (and wrapped by the
	// activity group) regardless of whether the model produced reasoning.
	if (memories.length > 0) {
		timeline.push({ kind: 'memory', memories });
	}

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
			pending: segment.pending
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

/** Nested activity-group rows stay closed until a segment/tool/subagent settles. */
export function isTimelineEntryToggleDisabled(entry: TimelineEntry): boolean {
	if (entry.kind === 'reasoning') return entry.pending === true;
	if (entry.kind === 'tool') return entry.tool.pending === true;
	if (entry.kind === 'subagent') {
		const subagent = entry.subagent;
		return (
			subagent.pending === true ||
			subagent.status === 'running' ||
			subagent.status === 'pending'
		);
	}
	return false;
}

/** Collapse pre-response timeline into one parent block once activity grows or final text exists. */
export function shouldGroupAssistantTimeline(
	assistant: AssistantItem,
	timeline: TimelineEntry[]
): boolean {
	if (timeline.length === 0) return false;
	for (const entry of timeline) {
		if (entry.kind === 'tool' && isPinnedJobProposalTool(entry.tool)) {
			return false;
		}
	}
	if (timeline.length >= 2) return true;
	return assistant.text.trim().length > 0;
}

/** Default parent activity group fold: collapsed; ChatThread auto-expands the active streaming turn. */
export function defaultActivityGroupExpanded(
	_assistant: AssistantItem,
	_streamingAssistantId: string | null,
	_sessionStreaming: boolean
): boolean {
	return false;
}

/** Whether an assistant turn is still in the pre-final / streaming response phase. */
export function isAssistantResponseActive(
	assistant: AssistantItem,
	streamingAssistantId: string | null,
	sessionStreaming: boolean
): boolean {
	return !assistant.text.trim() || (assistant.id === streamingAssistantId && sessionStreaming);
}

/** Default thinking fold: all segments start collapsed; user expands manually. */
export function defaultThinkingExpanded(
	_segmentIndex: number,
	_pending: boolean | undefined,
	_assistant: AssistantItem,
	_streamingAssistantId: string | null,
	_sessionStreaming: boolean
): boolean {
	return false;
}
