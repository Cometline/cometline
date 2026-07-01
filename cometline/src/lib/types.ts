import type { ImageAttachment, TokenUsage } from '$lib/generated/cometmind-api';

export type {
	CreateSessionRequest,
	ImageAttachment,
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

export type ProviderMethod = 'openai-compatible' | 'openai' | 'anthropic' | 'opencode-go' | 'codex';

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

export interface HeroComposerAppearance {
	glowColor: string;
	ringColor: string;
}

export interface CaretTrailSettings {
	enabled: boolean;
	intensity: number;
	speed: number;
}

export interface AppearanceSettings {
	heroComposer: HeroComposerAppearance;
	caretTrail: CaretTrailSettings;
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
	| 'focusSearch'
	| 'previousSession'
	| 'nextSession'
	| 'toggleWebPanel'
	| 'openWebPanel';

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
	webPanelWidth: number;
}

export interface ProviderSettings {
	providers: ProviderConfig[];
	activeProviderId: string;
	defaultModelId: string;
	defaultProviderId: string;
	appearance: AppearanceSettings;
	shortcuts: KeyboardShortcuts;
	app: AppSettings;
	cometmind: CometMindSettings;
}

export type SubagentProgressEntry =
	| { kind: 'stream'; channel: 'message' | 'thought' | 'plan'; text: string }
	| { kind: 'tool'; title: string; status: string }
	| { kind: 'status'; text: string };

export type ChatItem =
	| { id: string; type: 'user'; text: string; images?: ImageAttachment[]; reveal?: boolean }
	| {
			id: string;
			type: 'assistant';
			text: string;
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
			memories: {
				id: string;
				content: string;
				kind: string;
				similarity: number;
				effective_weight: number;
			}[];
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
