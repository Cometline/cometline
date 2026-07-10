import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { memoryToastStore } from './memory-toasts.svelte';

describe('memoryToastStore', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		for (const toast of memoryToastStore.toasts) memoryToastStore.dismiss(toast.id);
	});

	afterEach(() => {
		for (const toast of memoryToastStore.toasts) memoryToastStore.dismiss(toast.id);
		vi.useRealTimers();
	});

	it('creates action-specific toasts with a compact preview', () => {
		memoryToastStore.add([
			{
				action: 'create',
				kind: 'preference',
				content: '  Prefer   Traditional Chinese replies  '
			},
			{
				action: 'delete',
				kind: 'preference',
				content: 'Old preference'
			}
		]);

		expect(memoryToastStore.toasts.map(({ label, preview }) => ({ label, preview }))).toEqual([
			{ label: 'Memory saved', preview: 'Prefer Traditional Chinese replies' },
			{ label: 'Memory removed', preview: 'Old preference' }
		]);
	});

	it('expires toasts automatically', () => {
		memoryToastStore.add([{ action: 'update', kind: 'fact', content: 'Updated fact' }]);
		expect(memoryToastStore.toasts).toHaveLength(1);

		vi.advanceTimersByTime(5000);
		expect(memoryToastStore.toasts).toHaveLength(0);
	});
});
