import type { ChatItem } from '$lib/types';

export type SubagentChatItem = Extract<ChatItem, { type: 'subagent' }>;

/** General subagents run in-process; ACP delegates use OpenCode. */
export function isGeneralSubagent(subagent: SubagentChatItem): boolean {
	return subagent.agentName === 'cometmind';
}

/** True when the subagent hit its step limit rather than a hard error. */
export function isSubagentStepLimit(subagent: SubagentChatItem): boolean {
	if (subagent.status !== 'failed' && subagent.status !== 'incomplete') return false;
	const haystack = [
		subagent.summary ?? '',
		...subagent.progress
			.filter((entry) => entry.kind === 'status')
			.map((entry) => entry.text)
	]
		.join('\n')
		.toLowerCase();
	return haystack.includes('max steps exceeded') || haystack.includes('step limit reached');
}

/** Human-readable label for the subagent card header. */
export function subagentProgressLabel(subagent: SubagentChatItem): string {
	const toolCount = subagent.progress.filter((entry) => entry.kind === 'tool').length;
	const general = isGeneralSubagent(subagent);
	const stepLimit = isSubagentStepLimit(subagent);

	let prefix: string;
	if (subagent.status === 'incomplete' || stepLimit) {
		prefix = general ? 'CometMind · step limit' : 'OpenCode · step limit';
	} else if (subagent.status === 'failed') {
		prefix = general ? 'CometMind failed' : 'OpenCode failed';
	} else if (subagent.status === 'cancelled') {
		prefix = general ? 'CometMind cancelled' : 'OpenCode cancelled';
	} else if (general) {
		prefix = 'CometMind · research';
	} else {
		prefix = `OpenCode · ${subagent.agentName}`;
	}

	if (toolCount > 0) {
		return `${prefix} · ${toolCount} tool${toolCount === 1 ? '' : 's'}`;
	}
	return prefix;
}

/** Turn phase slug → readable chip label. */
export function formatSubagentPhaseLabel(phase: string): string {
	return phase.trim().replace(/_/g, ' ');
}
