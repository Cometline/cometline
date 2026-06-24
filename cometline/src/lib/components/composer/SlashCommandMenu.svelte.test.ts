// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import SlashCommandMenuHarness from './SlashCommandMenu.harness.svelte';

describe('SlashCommandMenu', () => {
	it('renders listbox with slash command options', () => {
		render(SlashCommandMenuHarness);
		expect(screen.getByRole('listbox', { name: 'Slash commands' })).toBeTruthy();
		expect(screen.getByRole('option', { name: '/model' })).toBeTruthy();
		expect(screen.getByRole('option', { name: '/change' })).toBeTruthy();
	});
});
