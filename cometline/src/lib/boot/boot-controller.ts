export interface BootInputs {
	connectionReady: boolean;
	settingsLoaded: boolean;
	introOpen: boolean;
	setupOpen: boolean;
	setupDismissed: boolean;
	setupCompleted: boolean;
	workspacePath: string;
	/** Mini and settings routes must not watch the workspace. */
	skipWatch: boolean;
}

export interface BootDeps {
	loadSessions: () => void;
	refreshModelLimits: () => void;
	ensureWorkspace: (path: string) => void;
	watchWorkspace: (path: string) => void;
	openSetup: () => void;
	clearBootMessage: () => void;
}

/**
 * Boot decisions live here so tests hit one interface.
 * Layout only adapts reactive inputs into sync().
 * Panel width is not written here — AppShell is the only writer.
 */
export function createBootController(deps: BootDeps) {
	let sessionsRequested = false;
	let setupRequested = false;
	let limitsConnectionSeenReady = false;
	let lastEnsuredWorkspace = '';
	let lastWatchedWorkspace = '';

	function sync(input: BootInputs) {
		if (input.connectionReady) {
			deps.clearBootMessage();
			if (!sessionsRequested) {
				sessionsRequested = true;
				deps.loadSessions();
			}
		}

		// Once per ready edge while settings are loaded. A later flap
		// (ready → not ready → ready) refreshes again. Plain sync() calls do not.
		if (input.connectionReady && input.settingsLoaded) {
			if (!limitsConnectionSeenReady) {
				limitsConnectionSeenReady = true;
				deps.refreshModelLimits();
			}
		} else if (!input.connectionReady) {
			limitsConnectionSeenReady = false;
		}

		if (
			input.settingsLoaded &&
			!input.introOpen &&
			!input.setupOpen &&
			!input.setupDismissed &&
			!input.setupCompleted &&
			!setupRequested
		) {
			setupRequested = true;
			deps.openSetup();
		}

		const path = input.workspacePath;
		if (path && path !== '/' && !input.skipWatch && path !== lastWatchedWorkspace) {
			lastWatchedWorkspace = path;
			deps.watchWorkspace(path);
		}

		if (input.connectionReady && path && path !== '/' && path !== lastEnsuredWorkspace) {
			lastEnsuredWorkspace = path;
			deps.ensureWorkspace(path);
		}
	}

	/** loadSessions failed. The next ready sync may try again. */
	function sessionsFailed() {
		sessionsRequested = false;
	}

	return { sync, sessionsFailed };
}
