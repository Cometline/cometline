import { describe, expect, it } from 'vitest';
import { itemsFromTranscript } from './chat-transcript';
import type { TranscriptItem } from '$lib/types';

describe('itemsFromTranscript', () => {
	it('maps user messages from transcript rows', () => {
		const items = itemsFromTranscript([
			{ type: 'user', text: 'Hi' },
			{ type: 'assistant', text: 'Hello' }
		]);
		expect(items).toHaveLength(2);
		expect(items[0]).toMatchObject({ type: 'user', text: 'Hi' });
		expect(items[1]).toMatchObject({ type: 'assistant', text: 'Hello' });
	});

	it('preserves user context chips from transcript rows', () => {
		const items = itemsFromTranscript([
			{
				type: 'user',
				text: 'look here',
				contexts: [
					{
						kind: 'file',
						title: 'notes.md',
						source: 'workspace-file:notes.md',
						role: 'viewing'
					},
					{
						kind: 'file',
						title: 'notes.md:2-3',
						source: 'workspace-file:notes.md#L2-L3'
					}
				]
			}
		]);
		expect(items[0]).toMatchObject({
			type: 'user',
			text: 'look here',
			contexts: [
				{
					kind: 'file',
					title: 'notes.md',
					source: 'workspace-file:notes.md',
					role: 'viewing'
				},
				{
					kind: 'file',
					title: 'notes.md:2-3',
					source: 'workspace-file:notes.md#L2-L3'
				}
			]
		});
	});

	it('merges assistant image refs into the assistant bubble', () => {
		const items = itemsFromTranscript([
			{ type: 'user', text: 'show me' },
			{
				type: 'assistant',
				media: [{ id: 'img1', media_type: 'image/png', alt: 'shot' }]
			},
			{ type: 'assistant', text: 'Here it is' }
		]);
		expect(items).toHaveLength(2);
		expect(items[1]).toMatchObject({
			type: 'assistant',
			text: 'Here it is',
			images: [{ id: 'img1', media_type: 'image/png', alt: 'shot' }]
		});
	});

	it('merges assistant video refs into the assistant bubble', () => {
		const items = itemsFromTranscript([
			{ type: 'user', text: 'clip' },
			{
				type: 'assistant',
				media: [{ id: 'vid1', media_type: 'video/mp4', alt: 'clip' }]
			},
			{ type: 'assistant', text: 'Here it is' }
		]);
		expect(items[1]).toMatchObject({
			type: 'assistant',
			text: 'Here it is',
			images: [{ id: 'vid1', media_type: 'video/mp4', alt: 'clip' }]
		});
	});

	it('keeps transcript errors attached to the assistant activity block', () => {
		const items = itemsFromTranscript([
			{ type: 'user', text: 'Run this' },
			{ type: 'tool', tool_name: 'read_file', tool_input: { path: 'README.md' } },
			{ type: 'error', text: 'provider failed' }
		]);

		expect(items.map((item) => item.type)).toEqual(['user', 'assistant', 'tool', 'error']);
		expect(items[1]).toMatchObject({ type: 'assistant', text: '' });
		expect(items[3]).toMatchObject({ type: 'error', text: 'provider failed' });
	});

	it('preserves buckets and infers them for historical bucketless memories', () => {
		const transcript = [
			{
				type: 'memory',
				memories: [
					{
						id: 'pref',
						content: 'Use concise replies',
						kind: 'preference',
						similarity: 1,
						effective_weight: 1
					},
					{
						id: 'task',
						content: 'Finished migration',
						kind: 'task_summary',
						similarity: 1,
						effective_weight: 1
					},
					{
						id: 'semantic',
						content: 'The app uses Svelte',
						kind: 'project',
						bucket: 'semantic',
						similarity: 0.9,
						effective_weight: 1
					}
				]
			}
		] as unknown as TranscriptItem[];

		const memory = itemsFromTranscript(transcript).find((item) => item.type === 'memory');
		expect(memory?.type).toBe('memory');
		if (memory?.type !== 'memory') return;
		expect(memory.memories.map((item) => item.bucket)).toEqual([
			'preference',
			'task_outcome',
			'semantic'
		]);
	});
});
