declare global {
	interface ProviderSettings {
		provider: string;
		baseURL: string;
		apiKey: string;
		selectedModel: string;
		models: string[];
	}

	interface Window {
		electronAPI?: {
			restartCometMind?: () => void;
			getWorkspacePath?: () => Promise<string>;
			getProviderSettings?: () => Promise<ProviderSettings>;
			fetchProviderModels?: (settings: ProviderSettings) => Promise<string[]>;
			saveProviderSettings?: (settings: ProviderSettings) => Promise<ProviderSettings>;
		};
	}
}

export {};
