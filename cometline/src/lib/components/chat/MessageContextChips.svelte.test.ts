// @vitest-environment jsdom
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import { afterEach, expect, it, vi } from 'vitest';
import MessageContextChips from './MessageContextChips.svelte';

afterEach(cleanup);

it.each([false, true])(
	'keeps context actions keyboard reachable (removable: %s)',
	async (removable) => {
		const onRemove = vi.fn();
		const onClearAll = vi.fn();
		const { getAllByRole, getByRole } = render(MessageContextChips, {
			contexts: [
				{ kind: 'file', title: 'a.md', source: 'workspace-file:a.md' },
				{ kind: 'file', title: 'b.md', source: 'workspace-file:b.md' }
			],
			removable,
			onRemove,
			onClearAll
		});

		for (const button of getAllByRole('button')) {
			expect(button.tabIndex).toBe(0);
			button.focus();
			expect(button).toHaveFocus();
		}
		if (removable) {
			await fireEvent.click(getByRole('button', { name: 'Remove a.md' }));
			expect(onRemove).toHaveBeenCalledWith(0);
			await fireEvent.click(getByRole('button', { name: 'Clear all' }));
			expect(onClearAll).toHaveBeenCalledOnce();
		}
	}
);
