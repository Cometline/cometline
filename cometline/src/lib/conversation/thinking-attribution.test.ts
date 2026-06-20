import { describe, expect, it } from 'vitest';
import { buildAssistantTimeline, buildThinkingAttribution } from './thinking-attribution';
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
