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

	it('renders a paste chip and the typed text beside it', () => {
		render(UserMessageRow, {
			props: {
				item: { id: 'u-mixed', type: 'user', text: '[Pasted ~12805 lines] hihi' },
				avatarSrc: '/project_avatar_192.png',
				copiedId: null,
				onCopyMessage: () => {}
			}
		});

		expect(screen.getByText('Pasted ~12805 lines')).toBeTruthy();
		expect(screen.getByText('hihi')).toBeTruthy();
		expect(screen.queryByText('[Pasted ~12805 lines]')).toBeNull();
	});

	it('shows a large user message as a chip and still copies the full text', async () => {
		const text = 'A long user message '.repeat(80);
		const onCopyMessage = vi.fn();
		render(UserMessageRow, {
			props: {
				item: { id: 'u-long', type: 'user', text },
				avatarSrc: '/project_avatar_192.png',
				copiedId: null,
				onCopyMessage
			}
		});

		expect(screen.getByText('Pasted text')).toBeTruthy();
		expect(screen.queryByText(text)).toBeNull();
		await fireEvent.click(screen.getByRole('button', { name: 'Copy message' }));
		expect(onCopyMessage).toHaveBeenCalledWith('u-long', text.trim());
	});

	it('toggles an overflowing message between collapsed and expanded', async () => {
		vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(640);
		vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(240);

		render(UserMessageRow, {
			props: {
				item: { id: 'u-long', type: 'user', text: 'Short but tall' },
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
