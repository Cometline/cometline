import { describe, expect, it } from 'vitest';
import { subagentProgressLabel } from '$lib/conversation/subagent-display';
import type { ChatItem } from '$lib/types';

function subagent(
	overrides: Partial<Extract<ChatItem, { type: 'subagent' }>> = {}
): Extract<ChatItem, { type: 'subagent' }> {
	return {
		id: 's1',
		type: 'subagent',
		childSessionId: 'child-1',
		purpose: 'Research task',
		agentName: 'cometmind',
		status: 'running',
		progress: [],
		pending: true,
		...overrides
	};
}

describe('subagentProgressLabel', () => {
	it('labels general subagents as CometMind research', () => {
		expect(subagentProgressLabel(subagent())).toBe('CometMind · research');
	});

	it('labels failed general subagents without OpenCode prefix', () => {
		expect(subagentProgressLabel(subagent({ status: 'failed', pending: false }))).toBe(
			'CometMind failed'
		);
	});

	it('labels step-limit subagents separately from hard failures', () => {
		expect(
			subagentProgressLabel(
				subagent({
					status: 'incomplete',
					pending: false,
					summary: 'Partial progress from tool calls:\n- web_fetch: ...'
				})
			)
		).toBe('CometMind · step limit');
	});

	it('labels ACP delegates as OpenCode', () => {
		expect(subagentProgressLabel(subagent({ agentName: 'opencode' }))).toBe(
			'OpenCode · opencode'
		);
	});

	it('counts tools in the label', () => {
		expect(
			subagentProgressLabel(
				subagent({
					progress: [
						{ kind: 'tool', title: 'web_fetch', status: 'running' },
						{ kind: 'tool', title: 'grep', status: 'running' }
					]
				})
			)
		).toBe('CometMind · research · 2 tools');
	});
});
