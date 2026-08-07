import { beforeEach, describe, expect, it } from 'vitest';
import { miniShellStore } from './mini-shell.svelte';

describe('mini shell new-session requests', () => {
	beforeEach(() => {
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
