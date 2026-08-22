import { beforeEach, describe, expect, it, vi } from 'vitest';
import { miniShellStore } from './mini-shell.svelte';

describe('mini shell opening transition', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		miniShellStore.resetOpening();
	});

	it('keeps the initial overlay visible until the opening has lasted 320ms', async () => {
		const opening = miniShellStore.startOpening();
		const finishing = miniShellStore.finishOpening(opening);

		expect(miniShellStore.opening).toBe(true);
		await vi.advanceTimersByTimeAsync(319);
		expect(miniShellStore.opening).toBe(true);
		await vi.advanceTimersByTimeAsync(1);
		await finishing;
		expect(miniShellStore.opening).toBe(false);
	});

	it('starts the minimum visible time when a prepared window is activated', async () => {
		miniShellStore.prepareOpening();
		await vi.advanceTimersByTimeAsync(5000);

		const opening = miniShellStore.startOpening();
		const finishing = miniShellStore.finishOpening(opening);
		await vi.advanceTimersByTimeAsync(319);
		expect(miniShellStore.opening).toBe(true);
		await vi.advanceTimersByTimeAsync(1);
		await finishing;
		expect(miniShellStore.opening).toBe(false);
	});

	it('does not let a stale opening completion hide a newer overlay', async () => {
		const first = miniShellStore.startOpening();
		miniShellStore.resetOpening();
		const second = miniShellStore.startOpening();
		const finishing = miniShellStore.finishOpening(first);

		await vi.runAllTimersAsync();
		await finishing;
		expect(miniShellStore.opening).toBe(true);

		const secondFinishing = miniShellStore.finishOpening(second);
		await vi.runAllTimersAsync();
		await secondFinishing;
		expect(miniShellStore.opening).toBe(false);
	});
});

describe('mini shell new-session requests', () => {
	beforeEach(() => {
		vi.useRealTimers();
		miniShellStore.clearNewSessionRequest();
	});

	it('only consumes the request for the matching session', () => {
		miniShellStore.requestNewSession('new-session');

		expect(miniShellStore.consumeNewSessionRequest('existing-session')).toBe(false);
		expect(miniShellStore.consumeNewSessionRequest('new-session')).toBe(true);
		expect(miniShellStore.consumeNewSessionRequest('new-session')).toBe(false);
	});

	it('does not clear a newer request for another session', () => {
		miniShellStore.requestNewSession('new-session');
		miniShellStore.clearNewSessionRequest('other-session');

		expect(miniShellStore.consumeNewSessionRequest('new-session')).toBe(true);
	});
});
