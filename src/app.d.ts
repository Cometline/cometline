declare global {
	type ProviderMethod = 'openai-compatible' | 'openai' | 'anthropic' | 'opencode-go';

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

	interface AppearanceSettings {
		heroComposer: HeroComposerAppearance;
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
		| 'nextSession';

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

	interface ProviderSettings {
		providers: ProviderConfig[];
		activeProviderId: string;
		appearance: AppearanceSettings;
		shortcuts: KeyboardShortcuts;
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

	interface Window {
		electronAPI?: {
			restartCometMind?: () => void;
			openExternal?: (url: string) => Promise<boolean>;
			getProviderSettings?: () => Promise<ProviderSettings>;
			fetchProviderModels?: (config: ProviderConfig) => Promise<string[]>;
			saveProviderSettings?: (
				settings: ProviderSettings,
				options?: { restartCometMind?: boolean }
			) => Promise<ProviderSettings>;
			setSidebarOpen?: (state: SidebarChromeState) => void;
			getFullScreen?: () => Promise<boolean>;
			onFullScreenChange?: (callback: (isFullScreen: boolean) => void) => () => void;
			getWorkspacePath?: () => Promise<string>;
			selectWorkspacePath?: () => Promise<string | null>;
			setWorkspacePath?: (workspacePath: string) => Promise<string>;
			getAppVersion?: () => Promise<string>;
			getUpdateState?: () => Promise<UpdateState>;
			checkForUpdates?: () => Promise<UpdateState>;
			installUpdate?: () => Promise<boolean>;
			onUpdateState?: (callback: (state: UpdateState) => void) => () => void;
			setShortcutCaptureActive?: (active: boolean) => void;
			setSessionNavigationSuspended?: (suspended: boolean) => void;
			onNavigateSession?: (callback: (direction: 'prev' | 'next') => void) => () => void;
		};
	}
}

export {};
