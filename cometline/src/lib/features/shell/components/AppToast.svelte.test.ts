// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import AppToast from './AppToast.svelte';
import { appToastStore } from '#lib/stores/app-toasts.svelte.js';

const compactDetail = 'Earlier messages were folded into the session summary.';

function clearToasts() {
	for (const toast of [...appToastStore.toasts]) appToastStore.dismiss(toast.id);
}

describe('AppToast', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
		clearToasts();
	});

	function renderToast() {
		vi.stubGlobal(
			'ResizeObserver',
			class {
				observe() {}
				unobserve() {}
				disconnect() {}
			}
		);
		return render(AppToast);
	}

	it('keeps the full compact notice in the toast instead of a single clipped line', () => {
		appToastStore.success('Context compacted', compactDetail);
		renderToast();

		expect(screen.getByText(compactDetail)).toBeTruthy();
		expect(
			screen.getByRole('button', { name: /Context compacted/ }).hasAttribute('disabled')
		).toBe(true);
	});

	it('holds an expanded toast until it is released', () => {
		vi.useFakeTimers();
		appToastStore.success('Context compacted', compactDetail);
		const id = appToastStore.toasts[0]?.id;
		expect(id).toBeTruthy();

		appToastStore.hold(id!);
		vi.advanceTimersByTime(11_000);
		expect(appToastStore.toasts.map((toast) => toast.id)).toContain(id);

		appToastStore.release(id!);
		vi.advanceTimersByTime(11_000);
		expect(appToastStore.toasts.map((toast) => toast.id)).not.toContain(id);
	});
});
