import type { CommandEnterSignal, ShortcutAction } from '#lib/keyboard-shortcuts.js';
import type {
	CustomPersona,
	FetchProviderModelsResult,
	ProviderConfig,
	ProviderSettings,
	TerminalSnapshot
} from '#lib/types.js';

export interface ElectronAPI {
	restartCometMind(): void;
	openExternal(url: string): Promise<boolean>;
	copyMediaFile(sessionId: string, mediaId: string): Promise<CopyMediaFileResult>;
	getProviderSettings(): Promise<ProviderSettings>;
	getCodexAuthStatus(): Promise<{
		authenticated: boolean;
		authPath: string;
		accountID?: string;
		error?: string;
	}>;
	startCodexLogin(): Promise<{ started: boolean; message: string }>;
	getXaiAuthStatus(): Promise<{ authenticated: boolean; authPath: string; error?: string }>;
	startXaiLogin(): Promise<{ started: boolean; message: string }>;
	readCursorMcpConfig(): Promise<CursorMcpConfigResult>;
	getDiscordGatewayStatus(): Promise<{ running: boolean; enabled: boolean }>;
	setDiscordGatewayEnabled(enabled: boolean): Promise<{ running: boolean; enabled: boolean }>;
	getOpenAtLogin(): Promise<OpenAtLoginState>;
	setOpenAtLogin(enabled: boolean): Promise<OpenAtLoginState>;
	getScreenCaptureAccess(): Promise<ScreenCaptureAccessState>;
	setScreenCapturePreferred(enabled: boolean): Promise<ScreenCaptureAccessState>;
	openScreenCaptureSettings(): Promise<boolean>;
	openSessionInMainWindow(sessionId: string): Promise<boolean>;
	openSettingsWindow(): Promise<boolean>;
	replayIntroInMainWindow(): Promise<boolean>;
	runSetupWizardInMainWindow(): Promise<boolean>;
	fetchProviderModels(config: ProviderConfig): Promise<FetchProviderModelsResult | string[]>;
	checkOllamaHealth(baseURL?: string): Promise<OllamaHealthResult>;
	listOllamaModels(baseURL?: string): Promise<{
		baseURL: string;
		models: Array<{ name: string; size?: number; digest?: string; modifiedAt?: string }>;
	}>;
	pullOllamaModel(payload: {
		baseURL?: string;
		catalogId?: string;
		modelName?: string;
	}): Promise<{ ok: boolean; model: string; models: Array<{ name: string; size?: number }> }>;
	cancelOllamaPull(): Promise<{ ok: boolean; cancelled: boolean; model?: string }>;
	onOllamaPullProgress(callback: (payload: OllamaPullProgress) => void): () => void;
	saveProviderSettings(
		settings: ProviderSettings,
		options?: {
			runtimeAction?: 'none' | 'reload' | 'restart' | 'gateway';
			restartCometMind?: boolean;
		}
	): Promise<SaveProviderSettingsResult>;
	setSidebarOpen(state: SidebarChromeState): void;
	getFullScreen(): Promise<boolean>;
	onFullScreenChange(callback: (isFullScreen: boolean) => void): () => void;
	onWorkspaceChanged(callback: (change: WorkspaceChange) => void): () => void;
	getWorkspacePath(): Promise<string>;
	selectWorkspacePath(): Promise<string | null>;
	browseWorkspacePath(): Promise<string | null>;
	selectBackupFolder(): Promise<SettingsFileResult>;
	setWorkspacePath(workspacePath: string): Promise<string>;
	watchWorkspace(workspacePath: string): Promise<void>;
	listRecentWorkspaces(): Promise<string[]>;
	removeRecentWorkspacePath(workspacePath: string): Promise<{ removed: boolean }>;
	filterExistingWorkspacePaths(paths: string[]): Promise<string[]>;
	pruneWorkspaceStore(): Promise<{ removedRecent: number; clearedCurrent: boolean }>;
	createPdfPreview(request: PdfPreviewRequest): Promise<PdfPreviewResult>;
	revokePdfPreview(token: string): Promise<void>;
	listTerminals(): Promise<TerminalSnapshot[]>;
	createTerminal(payload: TerminalCreatePayload): Promise<TerminalSnapshot>;
	writeTerminal(payload: { sessionId: string; data: string }): Promise<boolean>;
	resizeTerminal(payload: TerminalResizePayload): Promise<boolean>;
	terminateTerminal(sessionId: string): Promise<boolean>;
	removeTerminal(sessionId: string): Promise<boolean>;
	onTerminalData(callback: (payload: { sessionId: string; data: string }) => void): () => void;
	onTerminalExit(callback: (snapshot: TerminalSnapshot) => void): () => void;
	getAppVersion(): Promise<string>;
	getUpdateState(): Promise<UpdateState>;
	checkForUpdates(): Promise<UpdateState>;
	installUpdate(): Promise<boolean>;
	onUpdateState(callback: (state: UpdateState) => void): () => void;
	setShortcutCaptureActive(active: boolean): void;
	setSessionNavigationSuspended(suspended: boolean): void;
	setWorkspacePanelOpen(open: boolean): void;
	setInboxOpen(open: boolean): void;
	confirmCloseWindow(): void;
	onCloseWorkspacePanel(callback: () => void): () => void;
	onCloseInbox(callback: () => void): () => void;
	onRequestCloseWindow(callback: () => void): () => void;
	onRequestReload(callback: () => void): () => void;
	onNavigateSession(callback: (direction: 'prev' | 'next') => void): () => void;
	onShortcutAction(callback: (action: ShortcutAction) => void): () => void;
	onCommandEnter(callback: (signal: CommandEnterSignal) => void): () => void;
	onProviderSettingsChanged(callback: (settings: ProviderSettings) => void): () => void;
	onPersonaAvatarChanged(callback: (personaId: string) => void): () => void;
	onReplayIntro(callback: () => void): () => void;
	onRunSetupWizard(callback: () => void): () => void;
	onMiniWindowActivated(callback: () => void): () => void;
	getMiniWindowState(): Promise<MiniWindowState>;
	saveMiniWindowState(state: {
		sessionId?: string;
		lastActiveAt?: number;
	}): Promise<MiniWindowState>;
	notifyJob(payload: { title: string; body: string }): void;
	loadComposerHistory(): Promise<ComposerHistoryEntry[]>;
	appendComposerHistory(entry: ComposerHistoryEntry): Promise<ComposerHistoryResult>;
	listCustomPersonas(): Promise<CustomPersona[]>;
	saveCustomPersona(payload: {
		id?: string;
		name: string;
		soulMarkdown: string;
		avatarDataUrl?: string;
	}): Promise<SaveCustomPersonaResult>;
	deleteCustomPersona(id: string): Promise<DeleteCustomPersonaResult>;
	readPersonaAvatar(id: string): Promise<ReadPersonaAvatarResult>;
	readBuiltinSoul(personaId: string): Promise<ReadPersonaSoulResult>;
}

export type CopyMediaFileResult = { ok: true } | { ok: false; error: string };

export type CursorMcpConfigResult =
	| { ok: true; path: string; config: unknown }
	| { ok: false; error: string };

export interface OllamaHealthResult {
	ok: boolean;
	state: 'healthy' | 'missing' | 'unreachable';
	baseURL: string;
	version?: string;
	error?: string;
}

export interface OllamaPullProgress {
	model: string;
	status: string;
	total?: number;
	completed?: number;
	percent?: number;
	done?: boolean;
}

export interface WorkspaceChange {
	workspacePath: string;
	paths: string[];
	gitChanged: boolean;
}

export interface TerminalCreatePayload {
	sessionId: string;
	workspacePath: string;
	cols?: number;
	rows?: number;
}

export interface TerminalResizePayload {
	sessionId: string;
	cols: number;
	rows: number;
}

export interface ComposerHistoryEntry {
	display: string;
	timestamp: number;
	workspacePath: string;
	sessionId: string;
}

export type ComposerHistoryResult =
	| { ok: true; entries: ComposerHistoryEntry[] }
	| { ok: false; error: string; entries: ComposerHistoryEntry[] };

export interface MiniWindowState {
	sessionId: string;
	lastActiveAt: number;
	inactivityTimeoutMinutes: number;
}

export interface OpenAtLoginState {
	openAtLogin: boolean;
	status?: string;
	needsApproval?: boolean;
	openedSettings?: boolean;
	isDev?: boolean;
	message?: string;
}

export type ScreenCaptureAccessStatus =
	| 'granted'
	| 'denied'
	| 'not-determined'
	| 'restricted'
	| 'unknown'
	| 'unsupported';

export interface ScreenCaptureAccessState {
	preferred: boolean;
	status: ScreenCaptureAccessStatus;
	openedSettings?: boolean;
	message?: string;
}

export type SettingsFileResult =
	| { canceled: true }
	| { canceled: false; path: string; settings?: ProviderSettings };

export interface SidebarChromeState {
	open: boolean;
	duration: number;
}

export type UpdateStatus = 'idle' | 'checking' | 'downloading' | 'ready' | 'error';

export interface UpdateState {
	status: UpdateStatus;
	version?: string;
	percent?: number;
	message?: string;
	updatedAt?: number;
}

/**
 * Real outcome of applying a settings save to the running CometMind sidecar.
 * `action` distinguishes a confirmed in-place reload from a fallback/cold
 * restart so the UI can report what actually happened instead of assuming
 * every save silently succeeded.
 */
export interface RuntimeReloadOutcome {
	action: 'reload' | 'restart' | 'restart-fallback' | 'gateway';
	healthy: boolean;
	/** Present when action is 'restart-fallback': why the in-place reload failed. */
	error?: string;
}

export interface SaveProviderSettingsResult {
	settings: ProviderSettings;
	/** null when the save did not request any runtime action (e.g. shortcuts). */
	reload: RuntimeReloadOutcome | null;
}

export type PdfPreviewRequest =
	| { scope: 'workspace'; workspacePath: string; relativePath: string }
	| { scope: 'wiki'; relativePath: string };

export type PdfPreviewResult =
	| { ok: true; token: string; url: string }
	| { ok: false; error: string };

export type ReadPersonaSoulResult = { ok: true; content: string } | { ok: false; error: string };

export type ReadPersonaAvatarResult = { ok: true; dataUrl: string } | { ok: false; error: string };

export type SaveCustomPersonaResult =
	| { ok: true; persona: CustomPersona }
	| { ok: false; error: string };

export type DeleteCustomPersonaResult = { ok: true } | { ok: false; error: string };
