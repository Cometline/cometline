import { getActiveSessionId } from '#lib/active-session.js';

/** Default vs active workspace and the sidebar group ordering derived from them. */
export function createWorkspacePathsStore() {
	/** Persisted default workspace (Settings); survives restarts. */
	let defaultWorkspacePath = $state('/');
	/** Active workspace for composer, skills, and @mention context. */
	let workspacePath = $state('/');
	/** Sidebar group ordering; updated on explicit commit (click, send, workspace picker). */
	let sidebarOrderWorkspacePath = $state('/');
	/** When true, Discord gateway sessions are ordered before workspace groups. */
	let sidebarOrderDiscordActive = $state(false);

	return {
		get defaultWorkspacePath() {
			return defaultWorkspacePath;
		},
		get workspacePath() {
			return workspacePath;
		},
		get sidebarOrderWorkspacePath() {
			return sidebarOrderWorkspacePath;
		},
		get sidebarOrderDiscordActive() {
			return sidebarOrderDiscordActive;
		},
		/** Update persisted default; sync active when no session is open (home). */
		setDefaultWorkspacePath(path: string) {
			defaultWorkspacePath = path;
			if (!getActiveSessionId()) {
				workspacePath = path;
				sidebarOrderWorkspacePath = path;
				sidebarOrderDiscordActive = false;
			}
		},
		/** Boot: load default from Electron and align active workspace. */
		initializeDefaultWorkspace(path: string) {
			defaultWorkspacePath = path;
			workspacePath = path;
			sidebarOrderWorkspacePath = path;
			sidebarOrderDiscordActive = false;
		},
		setActiveWorkspacePath(path: string) {
			workspacePath = path;
		},
		setSidebarOrderWorkspacePath(path: string) {
			sidebarOrderWorkspacePath = path;
		},
		setSidebarOrderDiscordActive(active: boolean) {
			sidebarOrderDiscordActive = active;
		},
		/** Active workspace + sidebar order; does not touch default or Electron. */
		commitActiveWorkspace(path: string) {
			workspacePath = path;
			sidebarOrderWorkspacePath = path;
			sidebarOrderDiscordActive = false;
		},
		resetActiveToDefault() {
			workspacePath = defaultWorkspacePath;
			sidebarOrderWorkspacePath = defaultWorkspacePath;
			sidebarOrderDiscordActive = false;
		}
	};
}
