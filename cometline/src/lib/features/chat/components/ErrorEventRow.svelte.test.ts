// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ErrorEventRow from './ErrorEventRow.svelte';

describe('ErrorEventRow', () => {
	it('renders error text with alert styling', () => {
		render(ErrorEventRow, {
			props: {
				item: { id: 'err-1', type: 'error', text: 'Stream failed' }
			}
		});
		expect(screen.getByText('Error')).toBeTruthy();
		expect(screen.getByText('Stream failed')).toBeTruthy();
	});
});
