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
		| 'closeSettings';

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

	interface Window {
		electronAPI?: {
			restartCometMind?: () => void;
			getWorkspacePath?: () => Promise<string>;
			getProviderSettings?: () => Promise<ProviderSettings>;
			fetchProviderModels?: (config: ProviderConfig) => Promise<string[]>;
			saveProviderSettings?: (settings: ProviderSettings) => Promise<ProviderSettings>;
			setSidebarOpen?: (state: SidebarChromeState) => void;
			getFullScreen?: () => Promise<boolean>;
			onFullScreenChange?: (callback: (isFullScreen: boolean) => void) => () => void;
		};
	}
}

export {};
