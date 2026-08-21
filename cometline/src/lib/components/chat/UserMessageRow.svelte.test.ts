// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import UserMessageRow from './UserMessageRow.svelte';

describe('UserMessageRow', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('renders user message text', () => {
		render(UserMessageRow, {
			props: {
				item: { id: 'u1', type: 'user', text: 'Hello Cometline' },
				avatarSrc: '/project_avatar_192.png',
				copiedId: null,
				onCopyMessage: () => {}
			}
		});
		expect(screen.getByText('Hello Cometline')).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Expand user message' })).toBeNull();
	});

	it('toggles an overflowing message between collapsed and expanded', async () => {
		vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(640);
		vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(240);

		render(UserMessageRow, {
			props: {
				item: { id: 'u-long', type: 'user', text: 'A long user message '.repeat(80) },
				avatarSrc: '/project_avatar_192.png',
				copiedId: null,
				onCopyMessage: () => {}
			}
		});

		const expand = await screen.findByRole('button', { name: 'Expand user message' });
		expect(expand.getAttribute('aria-expanded')).toBe('false');
		expect(expand.getAttribute('aria-controls')).toBe('user-message-content-u-long');

		await fireEvent.click(expand);
		const collapse = await screen.findByRole('button', { name: 'Collapse user message' });
		expect(collapse.getAttribute('aria-expanded')).toBe('true');

		await fireEvent.click(collapse);
		expect(
			(await screen.findByRole('button', { name: 'Expand user message' })).getAttribute(
				'aria-expanded'
			)
		).toBe('false');
	});
});
