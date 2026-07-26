// @vitest-environment jsdom

import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import SessionFindBar from './SessionFindBar.svelte';
import type { SessionFindController } from '$lib/conversation/session-find.svelte';

function fakeController(): SessionFindController {
	return {
		open: true,
		query: 'needle',
		matchCount: 3,
		activeIndex: 1,
		focusRequestId: 1,
		openFind: vi.fn(),
		closeFind: vi.fn(),
		setQuery: vi.fn(),
		next: vi.fn(),
		previous: vi.fn(),
		observe: vi.fn(() => () => {}),
		rebuild: vi.fn()
	};
}

describe('SessionFindBar', () => {
	it('renders the result position and routes keyboard controls', async () => {
		const controller = fakeController();
		render(SessionFindBar, { controller });
		const input = screen.getByRole('searchbox', { name: 'Find text in current chat' });

		expect(screen.getByRole('search', { name: 'Find in current chat' })).toBeInTheDocument();
		expect(screen.getByText('2 / 3')).toBeInTheDocument();

		await fireEvent.input(input, { target: { value: 'updated' } });
		expect(controller.setQuery).toHaveBeenCalledWith('updated');
		await fireEvent.keyDown(input, { key: 'Enter' });
		expect(controller.next).toHaveBeenCalledOnce();
		await fireEvent.keyDown(input, { key: 'Enter', shiftKey: true });
		expect(controller.previous).toHaveBeenCalledOnce();
		await fireEvent.keyDown(input, { key: 'Escape' });
		expect(controller.closeFind).toHaveBeenCalledOnce();
	});
});
