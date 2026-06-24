// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';
import AssistantActivityGroupHarness from './AssistantActivityGroup.harness.svelte';

describe('AssistantActivityGroup', () => {
	it('toggles aria-expanded when parent toggle is clicked', async () => {
		let expanded = false;
		const onToggleParent = vi.fn(() => {
			expanded = !expanded;
		});

		const { container, rerender } = render(AssistantActivityGroupHarness, {
			props: {
				timeline: [{ kind: 'reasoning', segmentIndex: 0, text: 'Planning…' }],
				parentExpanded: expanded,
				onToggleParent
			}
		});

		const toggle = container.querySelector('.activity-group-toggle') as HTMLButtonElement;
		expect(toggle.getAttribute('aria-expanded')).toBe('false');

		await fireEvent.click(toggle);
		expect(onToggleParent).toHaveBeenCalledOnce();

		await rerender({
			timeline: [{ kind: 'reasoning', segmentIndex: 0, text: 'Planning…' }],
			parentExpanded: true,
			onToggleParent
		});
		expect(
			container.querySelector('.activity-group-toggle')?.getAttribute('aria-expanded')
		).toBe('true');
	});
});
