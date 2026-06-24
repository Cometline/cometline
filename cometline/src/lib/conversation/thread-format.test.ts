import { describe, expect, it } from 'vitest';
import {
	assistantThinkingWait,
	assistantWaitSeconds,
	formatToolDuration,
	toolDurationLabel,
	toolFoldLabel,
	usageText
} from './thread-format';
import type { ChatItem } from '$lib/stores/chat.svelte';

describe('formatToolDuration', () => {
	it('formats sub-second durations in ms', () => {
		expect(formatToolDuration(0)).toBe('1ms');
		expect(formatToolDuration(450)).toBe('450ms');
	});

	it('formats seconds with one decimal under 10s', () => {
		expect(formatToolDuration(1500)).toBe('1.5s');
	});

	it('formats whole seconds at 10s and above', () => {
		expect(formatToolDuration(12000)).toBe('12s');
	});
});

describe('toolDurationLabel', () => {
	const base = {
		id: 't1',
		type: 'tool' as const,
		toolName: 'read_file',
		input: '',
		output: ''
	};

	it('uses completed durationMs when present', () => {
		const item = { ...base, durationMs: 2500 };
		expect(toolDurationLabel(item, 1000)).toBe('2.5s');
	});

	it('computes elapsed time for pending tools', () => {
		const item = { ...base, pending: true, startedAt: 1000 };
		expect(toolDurationLabel(item, 3500)).toBe('2.5s');
	});

	it('returns empty string when no timing info', () => {
		expect(toolDurationLabel(base, 1000)).toBe('');
	});
});

describe('toolFoldLabel', () => {
	it('includes status and duration', () => {
		const item: Extract<ChatItem, { type: 'tool' }> = {
			id: 't1',
			type: 'tool',
			toolName: 'grep',
			input: '',
			output: '',
			pending: true,
			startedAt: 0
		};
		expect(toolFoldLabel(item, 1200)).toBe('grep → running · 1.2s');
	});

	it('omits duration when unavailable', () => {
		const item: Extract<ChatItem, { type: 'tool' }> = {
			id: 't1',
			type: 'tool',
			toolName: 'grep',
			input: '',
			output: '',
			error: 'failed'
		};
		expect(toolFoldLabel(item, 1000)).toBe('grep → fail');
	});
});

describe('usageText', () => {
	it('returns plain text when no usage', () => {
		const item: Extract<ChatItem, { type: 'status' }> = {
			id: 's1',
			type: 'status',
			text: 'Done'
		};
		expect(usageText(item)).toBe('Done');
	});

	it('appends token counts when usage is present', () => {
		const item: Extract<ChatItem, { type: 'status' }> = {
			id: 's1',
			type: 'status',
			text: 'Done',
			usage: { input_tokens: 100, output_tokens: 50, cache_read: 0, cache_write: 0 }
		};
		expect(usageText(item)).toBe('Done · 100 in / 50 out');
	});
});

describe('assistantWaitSeconds', () => {
	it('returns 0 when item is missing or has no pendingStartedAt', () => {
		expect(assistantWaitSeconds(undefined, 5000)).toBe(0);
		expect(assistantWaitSeconds({ id: 'a1', type: 'assistant', text: '' }, 5000)).toBe(0);
	});

	it('floors elapsed seconds from pendingStartedAt', () => {
		expect(
			assistantWaitSeconds(
				{ id: 'a1', type: 'assistant', text: '', pendingStartedAt: 1000 },
				5500
			)
		).toBe(4);
	});
});

describe('assistantThinkingWait', () => {
	it('delegates to assistantThinkingWaitStatus', () => {
		expect(
			assistantThinkingWait(
				{
					id: 'a1',
					type: 'assistant',
					text: '',
					activityPhase: 'compacting_context'
				},
				0
			)
		).toEqual({
			label: 'Thinking',
			detail: 'Summarizing earlier context…'
		});
	});
});
