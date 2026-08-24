import type { ElectronAPI } from '../electron/src/shared/api';

declare global {
	type ProviderMethod =
		| 'openai-compatible'
		| 'openai'
		| 'anthropic'
		| 'opencode-go'
		| 'codex'
		| 'xai'
		| 'ollama';

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
		presetId: 'blue' | 'rose' | 'custom';
		glowColor: string;
		ringColor: string;
		customPreset?: {
			glowColor: string;
			ringColor: string;
		};
	}

	interface CaretTrailSettings {
		enabled: boolean;
		intensity: number;
		speed: number;
	}

	type TerminalThemeId = 'cometline-dark' | 'dracula' | 'gruvbox-dark' | 'solarized-dark';

	interface TerminalAppearanceSettings {
		fontSize: number;
		theme: TerminalThemeId;
	}

	/** Chime when an agent run completes, is stopped, or fails. */
	interface ResponseCompleteSoundSettings {
		enabled: boolean;
		/** 0–1 linear gain applied to the HTMLAudioElement volume. */
		volume: number;
	}

	interface AppearanceSettings {
		heroComposer: HeroComposerAppearance;
		caretTrail: CaretTrailSettings;
		terminal: TerminalAppearanceSettings;
		responseCompleteSound: ResponseCompleteSoundSettings;
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
		| 'findInSession'
		| 'focusSearch'
		| 'previousSession'
		| 'nextSession'
		| 'toggleWorkspacePanel'
		| 'openWebSearch'
		| 'openGitPanel'
		| 'openWikiPanel'
		| 'openWorkspacePanel'
		| 'openFileSearch'
		| 'openTerminal'
		| 'navigateBack'
		| 'navigateForward'
		| 'openJobs'
		| 'openSkillDrafts'
		| 'openGallery'
		| 'openUsage'
		| 'openInbox'
		| 'recentSession';

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

	interface CustomPersona {
		id: string;
		name: string;
		avatarPath: string;
		soulPath: string;
		createdAt: number;
	}

	interface AppSettings {
		openAtLogin: boolean;
		hasSeenIntro: boolean;
		hasCompletedSetup: boolean;
		hasDismissedSetupWizard: boolean;
		personaId: string;
		personas: { custom: CustomPersona[] };
		miniWindowSessionId: string;
		miniWindowLastActiveAt: number;
		miniWindowInactivityTimeoutMinutes: number;
		workspacePanelWidth: number;
		workspacePanelRatio: number;
		confirmCloseOnCmdW: boolean;
		confirmBeforeDeletingChats: boolean;
		confirmBeforeDeletingMedia: boolean;
		fileSearchSource: 'wiki' | 'workspace';
		screenCapturePreferred: boolean;
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

	interface ScreenCaptureAccessState {
		preferred: boolean;
		status: string;
		openedSettings?: boolean;
		message?: string;
	}

	interface ProviderSettings {
		providers: ProviderConfig[];
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

	type CodingHarness = 'opencode' | 'claude' | 'codex';

	interface CometMindACPSettings {
		enabled: boolean;
		defaultHarness: CodingHarness;
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
		synthesisEnabled: boolean;
		synthesisProviderId: string;
		synthesisModel: string;
	}

	interface CometMindMemorySettings {
		enabled: boolean;
		autoExtract: boolean;
		autoRetrieve: boolean;
		maxRetrieved: number;
		taskOutcomeLimit: number;
		similarityThreshold: number;
		extractionProviderId: string;
		extractionModel: string;
		lifecycle: {
			decayHalfLifeDays: number;
			forgetThreshold: number;
			usageBoostFactor: number;
			maxUsageBoost: number;
			maxMemories: number;
			compactionTargetRatio: number;
			compactionOnExtract: boolean;
		};
		embedding: {
			providerId: string;
			provider: string;
			model: string;
			baseURL: string;
			apiKey: string;
		};
	}

	interface CometMindStorageBackupSettings {
		enabled: boolean;
		destinationDir: string;
		intervalHours: number;
		maxBackups: number;
	}

	interface CometMindStorageSettings {
		cleanupIntervalMinutes: number;
		retentionDays: number;
		detachedMediaRetentionDays: number;
		maxSessionsPerWorkspace: number;
		archivedMemoryPurgeDays: number;
		vacuumAfterPurge: boolean;
		toolOutputRetentionDays: number;
		agentTmpRetentionDays: number;
		backup: CometMindStorageBackupSettings;
	}

	interface CometMindJobsNotificationSettings {
		enabled: boolean;
		onClaimed: boolean;
		onCompleted: boolean;
		onReleased: boolean;
		onBlocked: boolean;
	}

	interface CometMindJobsSettings {
		notifications: CometMindJobsNotificationSettings;
		leaseMinutes: number;
		deletedPurgeDays: number;
		doneArchiveDays: number;
		archivedPurgeDays: number;
		staleReviewMinutes: number;
		maxConsecutiveFailures: number;
		retryCooldownMinutes: number;
		maxRetryCooldownMinutes: number;
		reconcileIntervalSeconds: number;
	}

	interface CometMindAutonomousJobsSettings {
		enabled: boolean;
		maxConcurrent: number;
		pollIntervalSeconds: number;
		maxStepsPerRun: number;
		providerId: string;
		modelId: string;
	}

	interface CometMindSchedulerSettings {
		enabled: boolean;
		pollIntervalSeconds: number;
	}

	interface CometMindGenerationModelSettings {
		providerId: string;
		model: string;
	}

	interface CometMindGenerationSettings {
		image: CometMindGenerationModelSettings;
		video: CometMindGenerationModelSettings;
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
		autonomy: CometMindAutonomousJobsSettings;
		scheduler: CometMindSchedulerSettings;
		generation: CometMindGenerationSettings;
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

	/**
	 * Real outcome of applying a settings save to the running CometMind sidecar.
	 * `action` distinguishes a confirmed in-place reload from a fallback/cold
	 * restart so the UI can report what actually happened instead of assuming
	 * every save silently succeeded.
	 */
	interface RuntimeReloadOutcome {
		action: 'reload' | 'restart' | 'restart-fallback' | 'gateway';
		healthy: boolean;
		/** Present when action is 'restart-fallback': why the in-place reload failed. */
		error?: string;
	}

	interface SaveProviderSettingsResult {
		settings: ProviderSettings;
		/** null when the save did not request any runtime action (e.g. shortcuts). */
		reload: RuntimeReloadOutcome | null;
	}

	type ReadWorkspaceFileResult =
		| { ok: true; kind: 'text'; content: string; extension: string }
		| { ok: true; kind: 'image'; mimeType: string; dataUrl: string }
		| { ok: false; error: string };

	type PdfPreviewRequest =
		| { scope: 'workspace'; workspacePath: string; relativePath: string }
		| { scope: 'wiki'; relativePath: string };
	type PdfPreviewResult = { ok: true; token: string; url: string } | { ok: false; error: string };

	type ReadPersonaSoulResult = { ok: true; content: string } | { ok: false; error: string };

	type ReadPersonaAvatarResult = { ok: true; dataUrl: string } | { ok: false; error: string };

	type SaveCustomPersonaResult =
		| { ok: true; persona: CustomPersona }
		| { ok: false; error: string };

	type DeleteCustomPersonaResult = { ok: true } | { ok: false; error: string };

	type TerminalStatus = 'running' | 'exited';

	interface TerminalSnapshot {
		sessionId: string;
		status: TerminalStatus;
		exitCode: number | null;
		generation: number;
		shell: string;
		output: string;
	}

	interface Window {
		electronAPI?: ElectronAPI;
	}
}

export {};
