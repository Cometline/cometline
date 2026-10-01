// @vitest-environment jsdom
import { render } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import MemoryCard from './MemoryCard.svelte';
import type { MemoryWire } from '$lib/types';

const memories: MemoryWire[] = [
	{
		id: 'preference-1',
		content: 'Use concise replies',
		kind: 'preference',
		bucket: 'preference',
		similarity: 1,
		effective_weight: 1
	},
	{
		id: 'task-1',
		content: 'Finished the memory migration',
		kind: 'task_outcome',
		bucket: 'task_outcome',
		similarity: 0.9,
		effective_weight: 1
	},
	{
		id: 'semantic-1',
		content: 'Cometline uses Svelte',
		kind: 'project',
		bucket: 'semantic',
		similarity: 0.8,
		effective_weight: 1
	}
];

describe('MemoryCard', () => {
	it('renders the three memory sections in contract order', () => {
		const { container, getByText } = render(MemoryCard, {
			props: { memories, expanded: true, onToggle: () => {} }
		});

		expect(getByText('Memories used · 3')).toBeTruthy();
		expect(
			Array.from(container.querySelectorAll('.memory-section-title')).map((node) =>
				node.textContent?.trim()
			)
		).toEqual(['User preferences', 'Relevant task outcomes', 'Semantic memories']);
		expect(getByText('project')).toBeTruthy();
		expect(getByText('Cometline uses Svelte')).toBeTruthy();
	});

	it('omits empty sections in content-only mode', () => {
		const { queryByText, getByText } = render(MemoryCard, {
			props: {
				memories: [memories[0]],
				expanded: true,
				contentOnly: true,
				onToggle: () => {}
			}
		});

		expect(getByText('User preferences')).toBeTruthy();
		expect(queryByText('Relevant task outcomes')).toBeNull();
		expect(queryByText('Semantic memories')).toBeNull();
	});
});
