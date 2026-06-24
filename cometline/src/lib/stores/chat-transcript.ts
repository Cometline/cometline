import type { ChatItem, Session, TranscriptItem } from '$lib/types';
import { getReasoningSegments } from '$lib/conversation/reasoning';
import { isSubagentStepLimit } from '$lib/conversation/subagent-display';
import { stripInlinedFileBlocks } from '$lib/messages/strip-inlined-files';

let nextLocalID = 0;

export function localID(prefix: string) {
	nextLocalID += 1;
	return `${prefix}-${Date.now()}-${nextLocalID}`;
}

function mapDelegationStatus(
	status: string | undefined
): Extract<ChatItem, { type: 'subagent' }>['status'] {
	switch (status) {
		case 'completed':
			return 'completed';
		case 'cancelled':
			return 'cancelled';
		case 'failed':
			return 'failed';
		case 'running':
			return 'running';
		default:
			return 'pending';
	}
}

function finalizeSubagentItem(
	item: Extract<ChatItem, { type: 'subagent' }>
): Extract<ChatItem, { type: 'subagent' }> {
	if (item.status === 'failed' && isSubagentStepLimit(item)) {
		return { ...item, status: 'incomplete' };
	}
	return item;
}

function subagentFromChild(
	child: Session,
	agentName = 'opencode'
): Extract<ChatItem, { type: 'subagent' }> {
	return finalizeSubagentItem({
		id: `subagent-${child.id}`,
		type: 'subagent',
		childSessionId: child.id,
		purpose: child.purpose ?? child.title ?? 'Delegated task',
		agentName,
		status: mapDelegationStatus(child.delegation_status),
		progress: [],
		summary: child.output_summary,
		pending: child.delegation_status === 'running'
	});
}

const SUBAGENT_SPAWN_TOOLS = new Set(['delegate_coding_task', 'spawn_general_agent']);

function agentNameForTool(toolName: string): string {
	return toolName === 'spawn_general_agent' ? 'cometmind' : 'opencode';
}

type ParsedSubagentBlock = {
	childSessionId: string;
	kind: string;
	status: string;
	summary: string;
};

function parseSubagentBlock(block: string): ParsedSubagentBlock | null {
	const lines = block.split('\n');
	let childSessionId = '';
	let kind = '';
	let status = '';
	const summaryLines: string[] = [];
	let inSummary = false;

	for (const line of lines) {
		if (line.startsWith('child_session_id:')) {
			childSessionId = line.slice('child_session_id:'.length).trim();
			continue;
		}
		if (line.startsWith('kind:')) {
			kind = line.slice('kind:'.length).trim();
			continue;
		}
		if (line.startsWith('status:')) {
			status = line.slice('status:'.length).trim();
			continue;
		}
		if (line.trim() === '' && !inSummary && childSessionId) {
			inSummary = true;
			continue;
		}
		if (inSummary) {
			summaryLines.push(line);
		}
	}

	if (!childSessionId) return null;
	return {
		childSessionId,
		kind,
		status,
		summary: summaryLines.join('\n').trim()
	};
}

function parseSubagentToolOutput(output: string | undefined): ParsedSubagentBlock[] {
	if (!output?.trim()) return [];
	if (output.includes('\n\nchild_session_id:')) {
		return output
			.split(/\n\n(?=child_session_id:)/)
			.map(parseSubagentBlock)
			.filter((block): block is ParsedSubagentBlock => block !== null);
	}
	const single = parseSubagentBlock(output);
	return single ? [single] : [];
}

function subagentFromParsed(
	block: ParsedSubagentBlock,
	toolName: string
): Extract<ChatItem, { type: 'subagent' }> {
	return finalizeSubagentItem({
		id: `subagent-${block.childSessionId}`,
		type: 'subagent',
		childSessionId: block.childSessionId,
		purpose: block.summary.split('\n')[0] || 'Delegated task',
		agentName: block.kind === 'general' ? 'cometmind' : agentNameForTool(toolName),
		status: mapDelegationStatus(block.status),
		progress: [],
		summary: block.summary,
		pending: block.status === 'running'
	});
}

export function mergeSubagents(items: ChatItem[], children: Session[]): ChatItem[] {
	const used = new Set<string>();
	const out: ChatItem[] = [];

	for (const item of items) {
		if (item.type === 'tool' && SUBAGENT_SPAWN_TOOLS.has(item.toolName)) {
			const match = children.find(
				(child) => !used.has(child.id) && item.output?.includes(child.id)
			);
			if (match) {
				used.add(match.id);
				out.push(subagentFromChild(match, agentNameForTool(item.toolName)));
				continue;
			}
			const parsed = parseSubagentToolOutput(item.output)[0];
			if (parsed) {
				used.add(parsed.childSessionId);
				out.push(subagentFromParsed(parsed, item.toolName));
				continue;
			}
		}

		if (item.type === 'tool' && item.toolName === 'wait_subagents') {
			out.push(item);
			for (const block of parseSubagentToolOutput(item.output)) {
				if (used.has(block.childSessionId)) continue;
				const child = children.find((c) => c.id === block.childSessionId);
				if (child) {
					used.add(child.id);
					out.push(
						subagentFromChild(
							child,
							block.kind === 'general' ? 'cometmind' : 'opencode'
						)
					);
				} else {
					used.add(block.childSessionId);
					out.push(subagentFromParsed(block, 'wait_subagents'));
				}
			}
			continue;
		}

		out.push(item);
	}

	const hasSubagentTools = out.some(
		(item) =>
			item.type === 'tool' &&
			(SUBAGENT_SPAWN_TOOLS.has(item.toolName) || item.toolName === 'wait_subagents')
	);
	if (hasSubagentTools) {
		for (const child of children) {
			if (!used.has(child.id)) {
				const agentName = child.subagent_kind === 'general' ? 'cometmind' : 'opencode';
				out.push(subagentFromChild(child, agentName));
			}
		}
	}

	return out;
}

export function itemsFromTranscript(transcriptItems: TranscriptItem[]): ChatItem[] {
	const out: ChatItem[] = [];
	let currentAssistant: Extract<ChatItem, { type: 'assistant' }> | null = null;

	function pushAssistant(index: number, text = '') {
		// Use a distinct prefix so an auto-created assistant placeholder never
		// collides with the `history-${index}` id of the row that triggered its
		// creation (e.g. a memory/tool row at the same loop index). Sharing an id
		// produces Svelte `each_key_duplicate` errors in the keyed transcript.
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: `history-assistant-${index}`,
			type: 'assistant',
			text
		};
		out.push(assistant);
		currentAssistant = assistant;
		return assistant;
	}

	function ensureAssistant(index: number) {
		return currentAssistant ?? pushAssistant(index, '');
	}

	function appendAssistantText(index: number, text: string) {
		if (!text) return;
		const assistant = ensureAssistant(index);
		assistant.text += text;
	}

	function appendReasoning(index: number, text: string) {
		if (!text) return;
		const assistant = ensureAssistant(index);
		const segments = [...getReasoningSegments(assistant.reasoning)];
		segments.push({ text, pending: false });
		assistant.reasoning = { segments };
	}

	for (let i = 0; i < transcriptItems.length; i++) {
		const item = transcriptItems[i];
		if (item.type === 'user' || item.type === 'system') {
			currentAssistant = null;
			out.push(itemFromTranscript(item, i));
			continue;
		}
		if (item.type === 'assistant') {
			appendAssistantText(i, item.text ?? '');
			continue;
		}
		if (item.type === 'reasoning') {
			appendReasoning(i, item.text ?? '');
			continue;
		}
		if (item.type === 'tool') {
			// The assistant step that owns this tool must precede it in the
			// array so the thinking-attribution scan (which walks forward and
			// attaches tools to the current assistant) groups it. Reasoning-less
			// turns persist their tools/memory before the assistant text row, so
			// without this the tools would render as loose, ungrouped pills after
			// a session reload.
			const host = ensureAssistant(i);
			const toolItem = itemFromTranscript(item, i);
			if (toolItem.type === 'tool') {
				toolItem.afterSegment = Math.max(
					0,
					getReasoningSegments(host.reasoning).length - 1
				);
			}
			out.push(toolItem);
			continue;
		}
		if (item.type === 'memory') {
			// Same rationale as tools: ensure the owning assistant exists first so
			// the memory card is grouped into the activity timeline rather than
			// floating as a standalone card on reload.
			ensureAssistant(i);
			out.push(itemFromTranscript(item, i));
			continue;
		}
		out.push(itemFromTranscript(item, i));
	}
	return out;
}

function itemFromTranscript(item: TranscriptItem, index: number): ChatItem {
	if (item.type === 'user')
		return {
			id: `history-${index}`,
			type: 'user',
			text: stripInlinedFileBlocks(item.text ?? ''),
			images: item.images
		};
	if (item.type === 'assistant')
		return { id: `history-${index}`, type: 'assistant', text: item.text ?? '' };
	if (item.type === 'system')
		return { id: `history-${index}`, type: 'status', text: item.text ?? '' };
	if (item.type === 'reasoning')
		return {
			id: `history-${index}`,
			type: 'assistant',
			text: '',
			reasoning: { segments: [{ text: item.text ?? '', pending: false }] }
		};
	if (item.type === 'memory')
		return {
			id: `history-${index}`,
			type: 'memory',
			memories: (item.memories ?? []).map((mem) => ({
				id: mem.id,
				content: mem.content,
				kind: mem.kind,
				similarity: mem.similarity,
				effective_weight: mem.effective_weight
			}))
		};
	return {
		id: `history-${index}`,
		type: 'tool',
		toolName: item.tool_name ?? '',
		input: item.tool_input,
		output: item.tool_error ? undefined : item.tool_output,
		error: item.tool_error ? item.tool_output : undefined,
		pending: false
	};
}
