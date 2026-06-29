declare global {
	type ProviderMethod = 'openai-compatible' | 'openai' | 'anthropic' | 'opencode-go' | 'codex';

	interface ProviderConfig {
		id: string;
		name: string;
		method: ProviderMethod;
		enabled: boolean;
		baseURL: string;
		apiKey: string;
		selectedModel: string;
		models: string[];
		enabledModels: string[];
	}

	interface FetchProviderModelsResult {
		models: string[];
	}

	interface HeroComposerAppearance {
		glowColor: string;
		ringColor: string;
	}

	interface CaretTrailSettings {
		enabled: boolean;
		intensity: number;
		speed: number;
	}

	interface AppearanceSettings {
		heroComposer: HeroComposerAppearance;
		caretTrail: CaretTrailSettings;
	}

	type ShortcutAction =
		| 'toggleSidebar'
		| 'openSettings'
		| 'newChat'
		| 'toggleMiniWindow'
		| 'stopResponse'
		| 'sendMessage'
		| 'insertNewline'
		| 'closeSettings'
		| 'focusSearch'
		| 'previousSession'
		| 'nextSession'
		| 'toggleWebPanel'
		| 'openWebPanel';

	interface ShortcutBinding {
		key: string;
		command?: boolean;
		ctrl?: boolean;
		meta?: boolean;
		alt?: boolean;
		shift?: boolean;
	}

	interface KeyboardShortcuts {
		[action: string]: ShortcutBinding;
	}

	interface AppSettings {
		openAtLogin: boolean;
		hasSeenIntro: boolean;
		hasCompletedSetup: boolean;
		hasDismissedSetupWizard: boolean;
		iconVariant: 'default' | 'man';
		miniWindowSessionId: string;
		miniWindowLastActiveAt: number;
		miniWindowInactivityTimeoutMinutes: number;
		webPanelWidth: number;
	}

	interface MiniWindowState {
		sessionId: string;
		lastActiveAt: number;
		inactivityTimeoutMinutes: number;
	}

	interface OpenAtLoginState {
		openAtLogin: boolean;
		status?: string;
		needsApproval?: boolean;
		openedSettings?: boolean;
		isDev?: boolean;
		message?: string;
	}

	interface ProviderSettings {
		providers: ProviderConfig[];
		activeProviderId: string;
		defaultModelId: string;
		defaultProviderId: string;
		appearance: AppearanceSettings;
		shortcuts: KeyboardShortcuts;
		app: AppSettings;
		cometmind: CometMindSettings;
	}

	type SettingsFileResult =
		| { canceled: true }
		| { canceled: false; path: string; settings?: ProviderSettings };

	interface CometMindACPSettings {
		command: string;
		args: string[];
		timeout: string;
	}

	interface CometMindDiscordGatewaySettings {
		enabled: boolean;
		botToken: string;
		botTokenEnv: string;
		providerId: string;
		modelId: string;
		allowedUsers: string[];
		allowedChannels: string[];
		requireMention: boolean;
		workspacePath: string;
	}

	interface CometMindSkillsSettings {
		enabled: boolean;
		roots: string[];
		includeOpenCode: boolean;
		includeClaude: boolean;
		mirrorToCometMind: boolean;
	}

	interface CometMindMemorySettings {
		extractionProviderId: string;
		extractionModel: string;
		embedding: {
			providerId: string;
			provider: string;
			model: string;
			baseURL: string;
			apiKey: string;
		};
	}

	interface CometMindStorageSettings {
		cleanupIntervalMinutes: number;
		retentionDays: number;
		maxSessionsPerWorkspace: number;
		archivedMemoryPurgeDays: number;
		deletedJobPurgeDays: number;
		vacuumAfterPurge: boolean;
	}

	interface CometMindJobsNotificationSettings {
		enabled: boolean;
		onClaimed: boolean;
		onCompleted: boolean;
		onReleased: boolean;
	}

	interface CometMindJobsSettings {
		notifications: CometMindJobsNotificationSettings;
		leaseMinutes: number;
		deletedPurgeDays: number;
		reconcileIntervalSeconds: number;
	}

	type MCPTransport = 'stdio' | 'http' | 'sse';

	interface MCPOAuthSettings {
		clientId?: string;
		scopes?: string[];
		authorizationUrl?: string;
		tokenUrl?: string;
	}

	interface MCPServerConfig {
		id: string;
		name: string;
		enabled: boolean;
		transport: MCPTransport;
		command?: string;
		args?: string[];
		env?: Record<string, string>;
		url?: string;
		headers?: Record<string, string>;
		oauth?: MCPOAuthSettings;
		allowedTools?: string[];
	}

	interface CometMindMCPSettings {
		enabled: boolean;
		servers: MCPServerConfig[];
	}

	interface CometMindSettings {
		systemPromptPath: string;
		maxTokens: number;
		logLevel: 'debug' | 'info' | 'warn' | 'error';
		contextWindowLimit: 128_000 | 256_000;
		titleProviderId: string;
		titleModelId: string;
		acp: CometMindACPSettings;
		skills: CometMindSkillsSettings;
		memory: CometMindMemorySettings;
		storage: CometMindStorageSettings;
		gateway: {
			discord: CometMindDiscordGatewaySettings;
		};
		mcp: CometMindMCPSettings;
		jobs: CometMindJobsSettings;
	}

	interface SidebarChromeState {
		open: boolean;
		duration: number;
	}

	type UpdateStatus = 'idle' | 'checking' | 'downloading' | 'ready' | 'error';

	interface UpdateState {
		status: UpdateStatus;
		version?: string;
		percent?: number;
		message?: string;
		updatedAt?: number;
	}

	type ReadWorkspaceFileResult =
		| { ok: true; kind: 'text'; content: string; extension: string }
		| { ok: true; kind: 'image'; mimeType: string; dataUrl: string }
		| { ok: false; error: string };

	interface Window {
		electronAPI?: {
			restartCometMind?: () => void;
			openExternal?: (url: string) => Promise<boolean>;
			getProviderSettings?: () => Promise<ProviderSettings>;
			getCodexAuthStatus?: () => Promise<{
				authenticated: boolean;
				authPath: string;
				accountID?: string;
				error?: string;
			}>;
			startCodexLogin?: () => Promise<{ started: boolean; message: string }>;
			getMcpOAuthStatus?: (serverId: string) => Promise<{
				authenticated: boolean;
				authPath: string;
				expiry?: string;
				error?: string;
			}>;
			startMcpOAuth?: (payload: {
				serverId: string;
				oauth: {
					clientId: string;
					scopes?: string[];
					authorizationUrl: string;
					tokenUrl: string;
				};
			}) => Promise<{ started: boolean; message: string }>;
			readCursorMcpConfig?: () => Promise<
				{ ok: true; path: string; config: unknown } | { ok: false; error: string }
			>;
			getDiscordGatewayStatus?: () => Promise<{ running: boolean; enabled: boolean }>;
			setDiscordGatewayEnabled?: (
				enabled: boolean
			) => Promise<{ running: boolean; enabled: boolean }>;
			getOpenAtLogin?: () => Promise<OpenAtLoginState>;
			setOpenAtLogin?: (enabled: boolean) => Promise<OpenAtLoginState>;
			openSessionInMainWindow?: (sessionId: string) => Promise<boolean>;
			fetchProviderModels?: (
				config: ProviderConfig
			) => Promise<FetchProviderModelsResult | string[]>;
			saveProviderSettings?: (
				settings: ProviderSettings,
				options?: { restartCometMind?: boolean }
			) => Promise<ProviderSettings>;
			exportProviderSettings?: () => Promise<SettingsFileResult>;
			importProviderSettings?: () => Promise<SettingsFileResult>;
			setSidebarOpen?: (state: SidebarChromeState) => void;
			getFullScreen?: () => Promise<boolean>;
			onFullScreenChange?: (callback: (isFullScreen: boolean) => void) => () => void;
			getWorkspacePath?: () => Promise<string>;
			selectWorkspacePath?: () => Promise<string | null>;
			setWorkspacePath?: (workspacePath: string) => Promise<string>;
			listRecentWorkspaces?: () => Promise<string[]>;
			removeRecentWorkspacePath?: (workspacePath: string) => Promise<{ removed: boolean }>;
			filterExistingWorkspacePaths?: (paths: string[]) => Promise<string[]>;
			pruneWorkspaceStore?: () => Promise<{ removedRecent: number; clearedCurrent: boolean }>;
			readWorkspaceFile?: (
				workspacePath: string,
				relativePath: string
			) => Promise<ReadWorkspaceFileResult>;
			getAppVersion?: () => Promise<string>;
			getUpdateState?: () => Promise<UpdateState>;
			checkForUpdates?: () => Promise<UpdateState>;
			installUpdate?: () => Promise<boolean>;
			onUpdateState?: (callback: (state: UpdateState) => void) => () => void;
			setShortcutCaptureActive?: (active: boolean) => void;
			setSessionNavigationSuspended?: (suspended: boolean) => void;
			setWebPanelOpen?: (open: boolean) => void;
			onCloseWebPanel?: (callback: () => void) => () => void;
			onToggleWebPanel?: (callback: () => void) => () => void;
			onOpenWebPanel?: (callback: () => void) => () => void;
			onNavigateSession?: (callback: (direction: 'prev' | 'next') => void) => () => void;
			onShortcutAction?: (
				callback: (action: import('$lib/keyboard-shortcuts').ShortcutAction) => void
			) => () => void;
			getMiniWindowState?: () => Promise<MiniWindowState>;
			saveMiniWindowState?: (state: {
				sessionId?: string;
				lastActiveAt?: number;
			}) => Promise<MiniWindowState>;
			notifyJob?: (payload: { title: string; body: string }) => void;
		};
	}
}

export {};
