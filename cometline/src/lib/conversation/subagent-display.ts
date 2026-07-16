import {
	AGENT_LABEL_CODING,
	AGENT_LABEL_RESEARCH,
	agentLabelForSessionKind
} from '$lib/tools/diff-artifact';
import type { ChatItem } from '$lib/types';

export type SubagentChatItem = Extract<ChatItem, { type: 'subagent' }>;

/** In-process CometMind subagents (research or coding); external harnesses use other names. */
export function isGeneralSubagent(subagent: SubagentChatItem): boolean {
	const name = subagent.agentName.trim().toLowerCase();
	return (
		name === AGENT_LABEL_RESEARCH ||
		name === AGENT_LABEL_CODING ||
		name.startsWith('cometmind')
	);
}

export function isCodingSubagent(subagent: SubagentChatItem): boolean {
	return subagent.agentName.trim().toLowerCase() === AGENT_LABEL_CODING;
}

function codingHarnessLabel(agentName: string): string {
	const normalized = agentName.trim().toLowerCase();
	if (normalized.includes('claude')) return 'Claude Code';
	if (normalized.includes('codex')) return 'Codex';
	if (!normalized || normalized.includes('opencode') || normalized === 'acp-agent') {
		return 'OpenCode';
	}
	return agentName;
}

/** True when the subagent hit its step limit rather than a hard error. */
export function isSubagentStepLimit(subagent: SubagentChatItem): boolean {
	if (subagent.status !== 'failed' && subagent.status !== 'incomplete') return false;
	const haystack = [
		subagent.summary ?? '',
		...subagent.progress.filter((entry) => entry.kind === 'status').map((entry) => entry.text)
	]
		.join('\n')
		.toLowerCase();
	return haystack.includes('max steps exceeded') || haystack.includes('step limit reached');
}

/** Human-readable label for the subagent card header. */
export function subagentProgressLabel(subagent: SubagentChatItem): string {
	const tools = subagent.progress.filter((entry) => entry.kind === 'tool');
	const toolTypeCount = tools.length;
	const toolCallCount = tools.reduce((count, tool) => count + tool.calls, 0);
	const general = isGeneralSubagent(subagent);
	const stepLimit = isSubagentStepLimit(subagent);
	const harness = general ? 'CometMind' : codingHarnessLabel(subagent.agentName);
	const suffix = !general && harness !== subagent.agentName ? ` · ${subagent.agentName}` : '';

	let prefix: string;
	if (subagent.status === 'incomplete' || stepLimit) {
		prefix = `${harness} · step limit`;
	} else if (subagent.status === 'failed') {
		prefix = `${harness} failed`;
	} else if (subagent.status === 'cancelled') {
		prefix = `${harness} cancelled`;
	} else if (general) {
		prefix = isCodingSubagent(subagent) ? 'CometMind · coding' : 'CometMind · research';
	} else {
		prefix = `${harness}${suffix}`;
	}

	if (toolTypeCount > 0) {
		return `${prefix} · ${toolTypeCount} tool type${toolTypeCount === 1 ? '' : 's'} · ${toolCallCount} call${toolCallCount === 1 ? '' : 's'}`;
	}
	return prefix;
}

/** Turn phase slug → readable chip label. */
export function formatSubagentPhaseLabel(phase: string): string {
	return phase.trim().replace(/_/g, ' ');
}

/** Resolve display agentName from tool output kind or session kind. */
export function resolveInProcessAgentName(kind: string | undefined): string {
	if (kind === 'coding' || kind === 'code') return AGENT_LABEL_CODING;
	if (kind === 'research' || kind === 'general' || !kind) return AGENT_LABEL_RESEARCH;
	return agentLabelForSessionKind(kind) || AGENT_LABEL_RESEARCH;
}
