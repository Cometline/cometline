// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/svelte';
import ThreadRowHarness from './ThreadRow.harness.svelte';

describe('ThreadRow', () => {
	it('applies variant and continuation-row classes', () => {
		const { container } = render(ThreadRowHarness);
		const row = container.querySelector('.thread-row');
		expect(row?.classList.contains('user-row')).toBe(true);
		expect(row?.classList.contains('continuation-row')).toBe(true);
	});
});
