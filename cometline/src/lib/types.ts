import type {
	ImageAttachment,
	MediaAttachment,
	MemoryWire,
	MessageContextRef,
	TokenUsage
} from '$lib/generated/cometmind-api';

export type {
	AgentMode,
	CreateSessionRequest,
	ImageAttachment,
	MediaAttachment,
	MemoryWire,
	MessageContextRef,
	PostMessageRequest,
	Session,
	SessionListResponse,
	StreamEvent,
	TokenUsage,
	TranscriptItem,
	TranscriptResponse,
	UpdateSessionRequest,
	Workspace
} from '$lib/generated/cometmind-api';
export type {
	Skill as SkillResource,
	ListSkillsResponse as SkillListResponse,
	SyncSkillsResponse as SkillSyncResponse
} from '$lib/generated/cometmind-api';

export type ProviderMethod =
	| 'openai-compatible'
	| 'openai'
	| 'anthropic'
	| 'opencode-go'
	| 'codex'
	| 'xai'
	| 'ollama';

export interface ProviderConfig {
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

export interface FetchProviderModelsResult {
	models: string[];
}

export type TerminalStatus = 'running' | 'exited';

export interface TerminalSnapshot {
	sessionId: string;
	status: TerminalStatus;
	exitCode: number | null;
	generation: number;
	shell: string;
	output: string;
}

export interface HeroComposerAppearance {
	presetId: 'blue' | 'rose' | 'custom';
	glowColor: string;
	ringColor: string;
	customPreset?: {
		glowColor: string;
		ringColor: string;
	};
}

export interface CaretTrailSettings {
	enabled: boolean;
	intensity: number;
	speed: number;
}

export type TerminalThemeId = 'cometline-dark' | 'dracula' | 'gruvbox-dark' | 'solarized-dark';

export interface TerminalAppearanceSettings {
	fontSize: number;
	theme: TerminalThemeId;
}

/** Chime when an agent run completes, is stopped, or fails. */
export interface ResponseCompleteSoundSettings {
	enabled: boolean;
	/** 0–1 linear gain applied to the HTMLAudioElement volume. */
	volume: number;
}

export interface AppearanceSettings {
	heroComposer: HeroComposerAppearance;
	caretTrail: CaretTrailSettings;
	terminal: TerminalAppearanceSettings;
	responseCompleteSound: ResponseCompleteSoundSettings;
}

export type ShortcutAction =
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
	| 'cycleReasoningEffort'
	| 'recentSession';

export interface ShortcutBinding {
	key: string;
	command?: boolean;
	ctrl?: boolean;
	meta?: boolean;
	alt?: boolean;
	shift?: boolean;
}

export type KeyboardShortcuts = Partial<Record<ShortcutAction, ShortcutBinding>>;

import type { CometMindSettings } from '$lib/cometmind-settings';

export interface CustomPersona {
	id: string;
	name: string;
	avatarPath: string;
	soulPath: string;
	createdAt: number;
}

export interface AppSettings {
	openAtLogin: boolean;
	hasSeenIntro: boolean;
	hasCompletedSetup: boolean;
	hasDismissedSetupWizard: boolean;
	personaId: string;
	personas: { custom: CustomPersona[] };
	miniWindowSessionId: string;
	miniWindowLastActiveAt: number;
	miniWindowInactivityTimeoutMinutes: number;
	/** Web/file panel width in px. 0 means use the default (50vw). */
	workspacePanelWidth: number;
	/**
	 * Preferred web/file panel width as a fraction of the content row.
	 * 0 means unset (derive from workspacePanelWidth or use the CSS default).
	 */
	workspacePanelRatio: number;
	/** When true, Cmd+W shows a Close confirmation before hiding the main window. */
	confirmCloseOnCmdW: boolean;
	/** When true, deleting a chat requires confirmation. */
	confirmBeforeDeletingChats: boolean;
	/** When true, deleting Gallery media requires confirmation. */
	confirmBeforeDeletingMedia: boolean;
	/** Default source for the ⌘P file search modal. */
	fileSearchSource: 'wiki' | 'workspace';
	/** User preference to enable screen / system-audio capture for presenting screenshots. */
	screenCapturePreferred: boolean;
}

export interface ProviderSettings {
	providers: ProviderConfig[];
	defaultModelId: string;
	defaultProviderId: string;
	appearance: AppearanceSettings;
	shortcuts: KeyboardShortcuts;
	app: AppSettings;
	cometmind: CometMindSettings;
}

export type SubagentProgressEntry =
	| { kind: 'stream'; channel: 'message' | 'thought' | 'plan'; text: string }
	| { kind: 'tool'; title: string; status: string; calls: number }
	| { kind: 'status'; text: string };

export type ChatItem =
	| {
			id: string;
			type: 'user';
			text: string;
			images?: ImageAttachment[];
			contexts?: MessageContextRef[];
			reveal?: boolean;
	  }
	| {
			id: string;
			type: 'assistant';
			text: string;
			images?: MediaAttachment[];
			pending?: boolean;
			pendingStartedAt?: number;
			activityPhase?: string;
			activityMessage?: string;
			reasoning?: {
				segments?: Array<{ text: string; pending?: boolean }>;
				/** @deprecated Legacy flat reasoning; normalized to segments by helpers. */
				text?: string;
				pending?: boolean;
			};
	  }
	| {
			id: string;
			type: 'tool';
			toolId?: string;
			toolName: string;
			input: unknown;
			output?: string;
			error?: string;
			pending?: boolean;
			startedAt?: number;
			durationMs?: number;
			/** Index of the reasoning segment this tool follows (0-based). */
			afterSegment?: number;
	  }
	| { id: string; type: 'status'; text: string; usage?: TokenUsage }
	| {
			id: string;
			type: 'memory';
			memories: MemoryWire[];
	  }
	| { id: string; type: 'error'; text: string }
	| {
			id: string;
			type: 'subagent';
			childSessionId: string;
			purpose: string;
			agentName: string;
			status: 'running' | 'completed' | 'failed' | 'cancelled' | 'pending' | 'incomplete';
			progress: SubagentProgressEntry[];
			summary?: string;
			pending?: boolean;
	  };
