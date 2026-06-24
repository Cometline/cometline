// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import UserMessageRow from './UserMessageRow.svelte';

describe('UserMessageRow', () => {
	it('renders user message text', () => {
		render(UserMessageRow, {
			props: {
				item: { id: 'u1', type: 'user', text: 'Hello Cometline' },
				iconVariant: 'default',
				copiedId: null,
				onCopyMessage: () => {}
			}
		});
		expect(screen.getByText('Hello Cometline')).toBeTruthy();
	});
});
