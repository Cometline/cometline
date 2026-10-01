// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import SettingsShortcutsPanel from './SettingsShortcutsPanel.svelte';
import { SHORTCUT_DEFINITIONS, defaultKeyboardShortcuts } from '$lib/keyboard-shortcuts';

describe('SettingsShortcutsPanel', () => {
	it('filters shortcuts by label, category, and current binding', async () => {
		render(SettingsShortcutsPanel, {
			props: {
				shortcuts: defaultKeyboardShortcuts(),
				onChange: vi.fn()
			}
		});

		expect(screen.getAllByRole('button', { name: 'Change' })).toHaveLength(
			SHORTCUT_DEFINITIONS.length
		);
		expect(screen.getByText(String(SHORTCUT_DEFINITIONS.length))).toBeTruthy();

		const filter = screen.getByRole('searchbox', { name: 'Filter shortcuts' });
		await fireEvent.input(filter, { target: { value: 'composer' } });

		const composerCount = SHORTCUT_DEFINITIONS.filter(
			(def) => def.category === 'composer'
		).length;
		expect(screen.getAllByRole('button', { name: 'Change' })).toHaveLength(composerCount);
		expect(screen.getByText(`${composerCount} / ${SHORTCUT_DEFINITIONS.length}`)).toBeTruthy();
		expect(screen.getByText('Send message')).toBeTruthy();
		expect(screen.queryByText('New chat')).toBeNull();

		await fireEvent.input(filter, { target: { value: 'escape' } });
		expect(screen.getByText('Close settings')).toBeTruthy();
		expect(screen.queryByText('Send message')).toBeNull();

		await fireEvent.input(filter, { target: { value: 'not-a-shortcut' } });
		expect(screen.queryAllByRole('button', { name: 'Change' })).toHaveLength(0);
		expect(screen.getByText('No shortcuts match “not-a-shortcut”.')).toBeTruthy();
	});
});
