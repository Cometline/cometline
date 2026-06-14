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

	interface ProviderSettings {
		providers: ProviderConfig[];
		activeProviderId: string;
	}

	interface Window {
		electronAPI?: {
			restartCometMind?: () => void;
			getWorkspacePath?: () => Promise<string>;
			getProviderSettings?: () => Promise<ProviderSettings>;
			fetchProviderModels?: (config: ProviderConfig) => Promise<string[]>;
			saveProviderSettings?: (settings: ProviderSettings) => Promise<ProviderSettings>;
		};
	}
}

export {};
