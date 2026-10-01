import { beforeEach, describe, expect, it, vi } from 'vitest';

const { goto } = vi.hoisted(() => ({ goto: vi.fn() }));

vi.mock('$app/navigation', () => ({ goto }));

import { gotoHome, gotoSession, isMiniRoutePath } from './session-route';

describe('session routes', () => {
	beforeEach(() => {
		goto.mockReset();
	});

	it('detects the mini window shell', () => {
		expect(isMiniRoutePath('/mini')).toBe(true);
		expect(isMiniRoutePath('/mini/session/abc')).toBe(true);
		expect(isMiniRoutePath('/minimal')).toBe(false);
		expect(isMiniRoutePath('/session/abc')).toBe(false);
	});

	it('opens sessions in the shell that is currently showing', async () => {
		await gotoSession('abc', '/session/old');
		expect(goto).toHaveBeenLastCalledWith('/session/abc');

		await gotoSession('abc', '/mini/session/old');
		expect(goto).toHaveBeenLastCalledWith('/mini/session/abc');
	});

	it('returns to the home route of the current shell', async () => {
		await gotoHome('/session/abc');
		expect(goto).toHaveBeenLastCalledWith('/');

		await gotoHome('/mini/session/abc');
		expect(goto).toHaveBeenLastCalledWith('/mini');
	});
});
