import { describe, expect, it } from 'vitest';
import {
	buildAssistantTimeline,
	buildThinkingAttribution,
	defaultActivityGroupExpanded,
	defaultThinkingExpanded,
	shouldGroupAssistantTimeline
} from './thinking-attribution';
import type { ChatItem } from '$lib/types';

describe('buildThinkingAttribution', () => {
	it('buffers memory from prior turns into the matching assistant block', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'first' },
			{
				id: 'm1',
				type: 'memory',
				memories: [
					{
						id: 'mem-1',
						kind: 'fact',
						content: 'alpha',
						similarity: 1,
						effective_weight: 1
					}
				]
			},
			{ id: 'a1', type: 'assistant', text: 'reply one' },
			{ id: 'u2', type: 'user', text: 'second' },
			{
				id: 'm2',
				type: 'memory',
				memories: [
					{
						id: 'mem-2',
						kind: 'fact',
						content: 'beta',
						similarity: 1,
						effective_weight: 1
					}
				]
			},
			{ id: 'a2', type: 'assistant', text: 'reply two' }
		];

		const { map, memoryIdsInBuffer } = buildThinkingAttribution(items);

		expect(memoryIdsInBuffer.has('m1')).toBe(true);
		expect(memoryIdsInBuffer.has('m2')).toBe(true);
		expect(map.get('a1')?.memories).toEqual([
			{ id: 'mem-1', kind: 'fact', content: 'alpha', similarity: 1, effective_weight: 1 }
		]);
		expect(map.get('a2')?.memories).toEqual([
			{ id: 'mem-2', kind: 'fact', content: 'beta', similarity: 1, effective_weight: 1 }
		]);
	});

	it('attaches memory injected after the assistant placeholder (live streaming order)', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'help me' },
			{ id: 'a1', type: 'assistant', text: '', pending: true },
			{
				id: 'm1',
				type: 'memory',
				memories: [
					{
						id: 'mem-1',
						kind: 'fact',
						content: 'alpha',
						similarity: 1,
						effective_weight: 1
					}
				]
			}
		];

		const { map, memoryIdsInBuffer } = buildThinkingAttribution(items);

		expect(memoryIdsInBuffer.has('m1')).toBe(true);
		expect(map.get('a1')?.memories).toEqual([
			{ id: 'mem-1', kind: 'fact', content: 'alpha', similarity: 1, effective_weight: 1 }
		]);
	});

	it('emits a memory timeline entry even when the assistant has no reasoning', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'remember this' },
			{
				id: 'm1',
				type: 'memory',
				memories: [
					{
						id: 'mem-1',
						kind: 'fact',
						content: 'alpha',
						similarity: 1,
						effective_weight: 1
					}
				]
			},
			{ id: 'a1', type: 'assistant', text: 'reply without reasoning' }
		];

		const attribution = buildThinkingAttribution(items);
		const timeline = buildAssistantTimeline('a1', items, attribution);

		expect(timeline.map((entry) => entry.kind)).toEqual(['memory']);
		if (timeline[0].kind === 'memory') {
			expect(timeline[0].memories).toEqual([
				{ id: 'mem-1', kind: 'fact', content: 'alpha', similarity: 1, effective_weight: 1 }
			]);
		}
	});

	it('leads the timeline with memory before reasoning when both exist', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'remember and think' },
			{
				id: 'm1',
				type: 'memory',
				memories: [
					{
						id: 'mem-1',
						kind: 'fact',
						content: 'alpha',
						similarity: 1,
						effective_weight: 1
					}
				]
			},
			{
				id: 'a1',
				type: 'assistant',
				text: 'done',
				reasoning: { segments: [{ text: 'thinking', pending: false }] }
			}
		];

		const attribution = buildThinkingAttribution(items);
		const timeline = buildAssistantTimeline('a1', items, attribution);

		expect(timeline.map((entry) => entry.kind)).toEqual(['memory', 'reasoning']);
	});

	it('does not buffer an orphan memory with no assistant in the turn', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'first' },
			{
				id: 'm1',
				type: 'memory',
				memories: [
					{
						id: 'mem-1',
						kind: 'fact',
						content: 'alpha',
						similarity: 1,
						effective_weight: 1
					}
				]
			},
			{ id: 'u2', type: 'user', text: 'second (no assistant followed m1)' },
			{ id: 'a1', type: 'assistant', text: 'reply two' }
		];

		const { memoryIdsInBuffer, map } = buildThinkingAttribution(items);

		expect(memoryIdsInBuffer.has('m1')).toBe(false);
		expect(map.get('a1')?.memories ?? []).toEqual([]);
	});

	it('buffers tools under the assistant in the same turn', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'run tool' },
			{ id: 'a1', type: 'assistant', text: '' },
			{
				id: 't1',
				type: 'tool',
				toolName: 'read_file',
				input: '{}',
				output: 'ok',
				pending: false
			},
			{ id: 'u2', type: 'user', text: 'next' },
			{ id: 'a2', type: 'assistant', text: 'done' }
		];

		const { map, toolIdsInBuffer } = buildThinkingAttribution(items);

		expect(toolIdsInBuffer.has('t1')).toBe(true);
		expect(map.get('a1')?.tools.map((tool) => tool.id)).toEqual(['t1']);
		expect(map.get('a2')?.tools ?? []).toEqual([]);
	});

	it('interleaves tools after the matching reasoning segment', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'run tool' },
			{
				id: 'a1',
				type: 'assistant',
				text: 'done',
				reasoning: {
					segments: [
						{ text: 'first thought', pending: false },
						{ text: 'second thought', pending: false }
					]
				}
			},
			{
				id: 't1',
				type: 'tool',
				toolName: 'read_file',
				input: {},
				output: 'ok',
				pending: false,
				afterSegment: 0
			},
			{
				id: 't2',
				type: 'tool',
				toolName: 'grep',
				input: {},
				error: 'not found',
				pending: false,
				afterSegment: 1
			}
		];

		const attribution = buildThinkingAttribution(items);
		const timeline = buildAssistantTimeline('a1', items, attribution);

		expect(timeline.map((entry) => entry.kind)).toEqual([
			'reasoning',
			'tool',
			'reasoning',
			'tool'
		]);
		if (timeline[0].kind === 'reasoning') {
			expect(timeline[0].text).toBe('first thought');
		}
		if (timeline[2].kind === 'reasoning') {
			expect(timeline[2].text).toBe('second thought');
		}
		if (timeline[1].kind === 'tool') {
			expect(timeline[1].tool.toolName).toBe('read_file');
		}
		if (timeline[3].kind === 'tool') {
			expect(timeline[3].tool.toolName).toBe('grep');
		}
	});

	it('resets pending memory at each user boundary', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'one' },
			{
				id: 'm1',
				type: 'memory',
				memories: [
					{
						id: 'mem-1',
						kind: 'fact',
						content: 'only turn one',
						similarity: 1,
						effective_weight: 1
					}
				]
			},
			{ id: 'a1', type: 'assistant', text: 'r1' },
			{ id: 'u2', type: 'user', text: 'two' },
			{ id: 'a2', type: 'assistant', text: 'r2' }
		];

		const { map } = buildThinkingAttribution(items);

		expect(map.get('a1')?.memories).toHaveLength(1);
		expect(map.get('a2')?.memories ?? []).toEqual([]);
	});

	it('buffers subagents under the assistant and places them after tools in the timeline', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'delegate this' },
			{
				id: 'a1',
				type: 'assistant',
				text: 'Here is the result.',
				reasoning: {
					segments: [{ text: 'planning delegation', pending: false }]
				}
			},
			{
				id: 't1',
				type: 'tool',
				toolName: 'delegate_coding_task',
				input: {},
				output: 'child-1',
				pending: false,
				afterSegment: 0
			},
			{
				id: 's1',
				type: 'subagent',
				childSessionId: 'child-1',
				purpose: 'Fix the bug',
				agentName: 'opencode',
				status: 'completed',
				progress: [],
				pending: false
			}
		];

		const attribution = buildThinkingAttribution(items);
		const timeline = buildAssistantTimeline('a1', items, attribution);

		expect(attribution.subagentIdsInBuffer.has('s1')).toBe(true);
		expect(timeline.map((entry) => entry.kind)).toEqual(['reasoning', 'tool', 'subagent']);
	});
});

describe('shouldGroupAssistantTimeline', () => {
	const assistantWithText: Extract<ChatItem, { type: 'assistant' }> = {
		id: 'a1',
		type: 'assistant',
		text: 'Here is the reply.'
	};
	const assistantNoText: Extract<ChatItem, { type: 'assistant' }> = {
		id: 'a1',
		type: 'assistant',
		text: ''
	};
	const reasoningEntry = {
		kind: 'reasoning' as const,
		segmentIndex: 0,
		text: 'planning',
		pending: false
	};
	const toolEntry = {
		kind: 'tool' as const,
		tool: {
			id: 't1',
			type: 'tool' as const,
			toolName: 'read_file',
			input: {},
			output: 'ok',
			pending: false
		}
	};

	it('does not group a single timeline entry before final text', () => {
		expect(shouldGroupAssistantTimeline(assistantNoText, [reasoningEntry])).toBe(false);
	});

	it('groups once the timeline has multiple entries, even without final text', () => {
		expect(shouldGroupAssistantTimeline(assistantNoText, [reasoningEntry, toolEntry])).toBe(
			true
		);
	});

	it('does not group when timeline is empty', () => {
		expect(shouldGroupAssistantTimeline(assistantWithText, [])).toBe(false);
	});

	it('groups when text exists and timeline has entries', () => {
		expect(shouldGroupAssistantTimeline(assistantWithText, [reasoningEntry])).toBe(true);
		expect(shouldGroupAssistantTimeline(assistantWithText, [reasoningEntry, toolEntry])).toBe(
			true
		);
	});

	it('groups refresh-like persisted data with all steps done', () => {
		const refreshedAssistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: 'Done.',
			reasoning: {
				segments: [
					{ text: 'first', pending: false },
					{ text: 'second', pending: false }
				]
			}
		};
		const timeline = [
			{ kind: 'reasoning' as const, segmentIndex: 0, text: 'first', pending: false },
			{ kind: 'reasoning' as const, segmentIndex: 1, text: 'second', pending: false },
			toolEntry
		];
		expect(shouldGroupAssistantTimeline(refreshedAssistant, timeline)).toBe(true);
	});
});

describe('propose_job pinning', () => {
	const proposeTool: Extract<ChatItem, { type: 'tool' }> = {
		id: 'pj1',
		type: 'tool',
		toolName: 'propose_job',
		input: { description: 'Fix auth' },
		output: '{"status":"awaiting_workspace","description":"Fix auth"}',
		pending: false
	};

	it('does not buffer completed propose_job', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'create a job' },
			{ id: 'a1', type: 'assistant', text: 'Please confirm below.' },
			proposeTool
		];
		const attribution = buildThinkingAttribution(items);
		expect(attribution.toolIdsInBuffer.has('pj1')).toBe(false);
	});

	it('does not group timeline when completed propose_job is present', () => {
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: 'Please confirm below.'
		};
		const timeline = [
			{
				kind: 'reasoning' as const,
				segmentIndex: 0,
				text: 'planning',
				pending: false
			},
			{ kind: 'tool' as const, tool: proposeTool }
		];
		expect(shouldGroupAssistantTimeline(assistant, timeline)).toBe(false);
	});
});

describe('defaultActivityGroupExpanded', () => {
	const assistant: Extract<ChatItem, { type: 'assistant' }> = {
		id: 'a1',
		type: 'assistant',
		text: 'Reply text'
	};

	it('defaults collapsed once response exists and turn is idle', () => {
		expect(defaultActivityGroupExpanded(assistant, null, false)).toBe(false);
		expect(defaultActivityGroupExpanded(assistant, 'other-id', false)).toBe(false);
	});

	it('defaults expanded while the same assistant is still streaming text', () => {
		expect(defaultActivityGroupExpanded(assistant, 'a1', true)).toBe(true);
	});

	it('defaults expanded when no response text yet', () => {
		const pendingAssistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: ''
		};
		expect(defaultActivityGroupExpanded(pendingAssistant, 'a1', true)).toBe(true);
		expect(defaultActivityGroupExpanded(pendingAssistant, null, false)).toBe(true);
	});

	it('defaults collapsed after reload when all steps are done', () => {
		const refreshedAssistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: 'Done.',
			reasoning: { segments: [{ text: 'thought', pending: false }] }
		};
		expect(defaultActivityGroupExpanded(refreshedAssistant, null, false)).toBe(false);
	});
});

describe('defaultThinkingExpanded', () => {
	const assistant: Extract<ChatItem, { type: 'assistant' }> = {
		id: 'a1',
		type: 'assistant',
		text: ''
	};

	it('auto-expands the first segment while the response is active', () => {
		expect(defaultThinkingExpanded(0, true, assistant, 'a1', true)).toBe(true);
		expect(defaultThinkingExpanded(0, false, assistant, 'a1', true)).toBe(true);
		expect(defaultThinkingExpanded(1, true, assistant, 'a1', true)).toBe(false);
	});

	it('keeps the first segment open while the same turn is still active', () => {
		expect(defaultThinkingExpanded(0, false, assistant, 'a1', true)).toBe(true);
	});

	it('folds all segments once the final response is idle', () => {
		const done: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: 'Final answer.'
		};
		expect(defaultThinkingExpanded(0, false, done, null, false)).toBe(false);
		expect(defaultThinkingExpanded(0, true, done, null, false)).toBe(false);
	});
});
