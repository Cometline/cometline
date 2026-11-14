// @vitest-environment jsdom
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SidebarSearch from './SidebarSearch.svelte';

describe('SidebarSearch new chat button', () => {
	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();
	});

	it('calls onNewChat with no click event argument', async () => {
		const onNewChat = vi.fn();
		const { getByRole } = render(SidebarSearch, { onNewChat });
		await fireEvent.click(getByRole('button', { name: 'New chat' }));
		expect(onNewChat).toHaveBeenCalledOnce();
		expect(onNewChat.mock.calls[0]).toEqual([]);
	});
});
