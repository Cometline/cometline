import { readHasSeenIntroSync } from '$lib/stores/settings.svelte';

/** Window chrome: sidebar, overlays (settings / intro / setup), composer phase, boot status. */
export function createShellChromeStore() {
	let sidebarOpen = $state(true);
	let settingsOpen = $state(false);
	// Read localStorage synchronously so the very first rendered frame already
	// has the correct value — no IPC round-trip needed. New users (no stored
	// flag) get true; returning users get false. Zero flash either way.
	let introOpen = $state(!readHasSeenIntroSync());
	// Setup wizard: separate from the cinematic intro. A new user who hasn't
	// completed provider configuration sees the wizard after the intro ends.
	let setupOpen = $state(false);
	let composerPhase = $state<'centered' | 'docked'>('centered');
	let bootMessage = $state('');
	let fullscreen = $state(false);

	return {
		get sidebarOpen() {
			return sidebarOpen;
		},
		get fullscreen() {
			return fullscreen;
		},
		get settingsOpen() {
			return settingsOpen;
		},
		get introOpen() {
			return introOpen;
		},
		get setupOpen() {
			return setupOpen;
		},
		get composerPhase() {
			return composerPhase;
		},
		get bootMessage() {
			return bootMessage;
		},
		setBootMessage(message: string) {
			bootMessage = message;
		},
		setFullscreen(value: boolean) {
			fullscreen = value;
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
		openIntro() {
			introOpen = true;
		},
		closeIntro() {
			introOpen = false;
		},
		openSetup() {
			setupOpen = true;
		},
		closeSetup() {
			setupOpen = false;
		},
		dockComposer() {
			composerPhase = 'docked';
		},
		centerComposer() {
			composerPhase = 'centered';
		}
	};
}
