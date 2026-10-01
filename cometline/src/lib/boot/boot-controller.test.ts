import { describe, expect, it, vi } from 'vitest';
import { createBootController, type BootInputs } from './boot-controller';

function deps() {
	return {
		loadSessions: vi.fn(),
		refreshModelLimits: vi.fn(),
		ensureWorkspace: vi.fn(),
		watchWorkspace: vi.fn(),
		openSetup: vi.fn(),
		clearBootMessage: vi.fn()
	};
}

function input(overrides: Partial<BootInputs> = {}): BootInputs {
	return {
		connectionReady: false,
		settingsLoaded: false,
		introOpen: false,
		setupOpen: false,
		setupDismissed: false,
		setupCompleted: false,
		workspacePath: '/ws',
		skipWatch: false,
		...overrides
	};
}

describe('createBootController', () => {
	it('does not load sessions before the connection is ready', () => {
		const calls = deps();
		const boot = createBootController(calls);
		boot.sync(input());
		expect(calls.loadSessions).not.toHaveBeenCalled();
		expect(calls.clearBootMessage).not.toHaveBeenCalled();
	});

	it('loads sessions and clears the boot message once across two ready ticks', () => {
		const calls = deps();
		const boot = createBootController(calls);
		boot.sync(input({ connectionReady: true }));
		boot.sync(input({ connectionReady: true }));
		expect(calls.loadSessions).toHaveBeenCalledTimes(1);
		expect(calls.clearBootMessage).toHaveBeenCalledTimes(2);
	});

	it('opens setup once when intro is closed and setup was not dismissed', () => {
		const calls = deps();
		const boot = createBootController(calls);
		boot.sync(input({ settingsLoaded: true }));
		boot.sync(input({ settingsLoaded: true }));
		expect(calls.openSetup).toHaveBeenCalledTimes(1);
	});

	it('does not open setup when dismissed, completed, or intro is still open', () => {
		const dismissed = deps();
		createBootController(dismissed).sync(input({ settingsLoaded: true, setupDismissed: true }));
		expect(dismissed.openSetup).not.toHaveBeenCalled();

		const completed = deps();
		createBootController(completed).sync(input({ settingsLoaded: true, setupCompleted: true }));
		expect(completed.openSetup).not.toHaveBeenCalled();

		const intro = deps();
		createBootController(intro).sync(input({ settingsLoaded: true, introOpen: true }));
		expect(intro.openSetup).not.toHaveBeenCalled();
	});

	it('ensures a workspace once per path and not again when the connection flaps', () => {
		const calls = deps();
		const boot = createBootController(calls);
		boot.sync(input({ connectionReady: true, workspacePath: '/a' }));
		boot.sync(input({ connectionReady: false, workspacePath: '/a' }));
		boot.sync(input({ connectionReady: true, workspacePath: '/a' }));
		boot.sync(input({ connectionReady: true, workspacePath: '/b' }));
		expect(calls.ensureWorkspace).toHaveBeenCalledTimes(2);
		expect(calls.ensureWorkspace).toHaveBeenNthCalledWith(1, '/a');
		expect(calls.ensureWorkspace).toHaveBeenNthCalledWith(2, '/b');
	});

	it('skips watchWorkspace for the mini route and for /', () => {
		const calls = deps();
		const boot = createBootController(calls);
		boot.sync(input({ skipWatch: true, workspacePath: '/ws' }));
		boot.sync(input({ workspacePath: '/' }));
		expect(calls.watchWorkspace).not.toHaveBeenCalled();
	});

	it('does not refresh model limits before settings are loaded', () => {
		const calls = deps();
		const boot = createBootController(calls);
		boot.sync(input({ connectionReady: true, settingsLoaded: false }));
		expect(calls.refreshModelLimits).not.toHaveBeenCalled();
		boot.sync(input({ connectionReady: true, settingsLoaded: true }));
		boot.sync(input({ connectionReady: true, settingsLoaded: true }));
		expect(calls.refreshModelLimits).toHaveBeenCalledTimes(1);
	});

	it('lets a failed session load retry on the next ready sync', () => {
		const calls = deps();
		const boot = createBootController(calls);
		boot.sync(input({ connectionReady: true }));
		boot.sessionsFailed();
		boot.sync(input({ connectionReady: true }));
		expect(calls.loadSessions).toHaveBeenCalledTimes(2);
	});
});
