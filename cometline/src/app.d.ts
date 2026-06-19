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
		| 'stopResponse'
		| 'sendMessage'
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
		iconVariant: 'default' | 'man';
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
		interactive: boolean;
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
		retentionDays: number;
		maxSessionsPerWorkspace: number;
		archivedMemoryPurgeDays: number;
		vacuumAfterPurge: boolean;
	}

	interface CometMindSettings {
		systemPromptPath: string;
		maxTokens: number;
		titleProviderId: string;
		titleModelId: string;
		acp: CometMindACPSettings;
		skills: CometMindSkillsSettings;
		memory: CometMindMemorySettings;
		storage: CometMindStorageSettings;
		gateway: {
			discord: CometMindDiscordGatewaySettings;
		};
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
			getDiscordGatewayStatus?: () => Promise<{ running: boolean; enabled: boolean }>;
			setDiscordGatewayEnabled?: (
				enabled: boolean
			) => Promise<{ running: boolean; enabled: boolean }>;
			getOpenAtLogin?: () => Promise<OpenAtLoginState>;
			setOpenAtLogin?: (enabled: boolean) => Promise<OpenAtLoginState>;
			fetchProviderModels?: (config: ProviderConfig) => Promise<string[]>;
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
		};
	}
}

export {};

declare namespace svelteHTML {
	interface IntrinsicElements {
		webview: import('svelte/elements').HTMLAttributes<HTMLElement> & {
			src?: string;
			sandbox?: string;
			partition?: string;
		};
	}
}
