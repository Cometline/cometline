// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import ErrorBanner from './ErrorBanner.svelte';

describe('ErrorBanner', () => {
	it('renders message with alert role', () => {
		render(ErrorBanner, {
			props: { message: 'Something went wrong' }
		});
		expect(screen.getByRole('alert')).toBeTruthy();
		expect(screen.getByText('Something went wrong')).toBeTruthy();
	});

	it('calls onDismiss when dismiss button is clicked', async () => {
		const onDismiss = vi.fn();
		render(ErrorBanner, {
			props: { message: 'Retry later', onDismiss }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dismiss error' }));
		expect(onDismiss).toHaveBeenCalledOnce();
	});
});
