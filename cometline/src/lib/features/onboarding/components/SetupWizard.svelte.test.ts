// @vitest-environment jsdom
import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import SetupWizard from './SetupWizard.svelte';

vi.mock('$lib/client/cometmind', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/client/cometmind')>();
	return { ...actual, getMemorySettings: vi.fn(async () => actual.defaultMemorySettings()) };
});

describe('SetupWizard', () => {
	it('walks from provider selection to the account step', async () => {
		render(SetupWizard);

		expect(screen.getByRole('dialog', { name: 'Welcome to Cometline' })).toBeTruthy();
		expect(screen.getByText('Choose a provider — step 1 of 6')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Next' }));

		expect(screen.getByText('Connect your account — step 2 of 6')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Back' })).toBeTruthy();
		expect(screen.getByLabelText('Selected providers')).toBeTruthy();
	});
});
