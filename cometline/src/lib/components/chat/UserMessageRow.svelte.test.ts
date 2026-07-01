// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import UserMessageRow from './UserMessageRow.svelte';

describe('UserMessageRow', () => {
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
	});
});
