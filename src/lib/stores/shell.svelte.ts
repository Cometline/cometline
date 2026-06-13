function createShellStore() {
	let sidebarOpen = $state(true);
	let settingsOpen = $state(false);
	let composerPhase = $state<'centered' | 'docked'>('centered');
	let workspacePath = $state('/');
	let bootMessage = $state('');

	return {
		get sidebarOpen() {
			return sidebarOpen;
		},
		get settingsOpen() {
			return settingsOpen;
		},
		get composerPhase() {
			return composerPhase;
		},
		get workspacePath() {
			return workspacePath;
		},
		get bootMessage() {
			return bootMessage;
		},
		setWorkspacePath(path: string) {
			workspacePath = path;
		},
		setBootMessage(message: string) {
			bootMessage = message;
		},
		toggleSidebar() {
			sidebarOpen = !sidebarOpen;
		},
		openSidebar() {
			sidebarOpen = true;
		},
		closeSidebar() {
			sidebarOpen = false;
		},
		openSettings() {
			settingsOpen = true;
		},
		closeSettings() {
			settingsOpen = false;
		},
		dockComposer() {
			composerPhase = 'docked';
		},
		centerComposer() {
			composerPhase = 'centered';
		}
	};
}

export const shellStore = createShellStore();
