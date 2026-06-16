export interface Session {
	id: string;
	workspace_id: string;
	workspace_path: string;
	title: string;
	model_id: string;
	provider_id: string;
	status: 'active' | 'archived';
	token_usage: TokenUsage;
	parent_session_id?: string;
	purpose?: string;
	delegation_status?: string;
	output_summary?: string;
	pending_question?: string;
	created_at: number;
	updated_at: number;
}

export interface TokenUsage {
	input_tokens: number;
	output_tokens: number;
	cache_read: number;
	cache_write: number;
}

export interface CreateSessionRequest {
	workspace_id?: string;
	workspace_path?: string;
	model_id?: string;
	provider_id?: string;
}

export interface UpdateSessionRequest {
	model_id: string;
	provider_id: string;
}

export interface Workspace {
	id: string;
	path: string;
}

export interface SkillResource {
	name: string;
	description: string;
	path: string;
	source: string;
	internal: boolean;
}

export interface SkillListResponse {
	skills: SkillResource[];
	errors?: string[];
}

export interface SkillSyncResponse {
	created: string[];
	skipped: string[];
	errors?: string[];
}

export interface PostMessageRequest {
	text: string;
	images?: ImageAttachment[];
}

export interface ImageAttachment {
	id?: string;
	media_type: string;
	data: string;
	name?: string;
	size?: number;
}

export type ProviderMethod = 'openai-compatible' | 'openai' | 'anthropic' | 'opencode-go';

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
	| 'stopResponse'
	| 'sendMessage'
	| 'insertNewline'
	| 'closeSettings'
	| 'focusSearch'
	| 'previousSession'
	| 'nextSession'
	| 'toggleWebPanel';

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

export interface AppSettings {
	openAtLogin: boolean;
}

export interface ProviderSettings {
	providers: ProviderConfig[];
	activeProviderId: string;
	appearance: AppearanceSettings;
	shortcuts: KeyboardShortcuts;
	app: AppSettings;
	cometmind: CometMindSettings;
}

export interface SessionListResponse {
	sessions: Session[];
}

export interface TranscriptResponse {
	session_id: string;
	items: TranscriptItem[];
}

export type TranscriptItem =
	| { type: 'user'; text: string; images?: ImageAttachment[] }
	| { type: 'assistant'; text: string }
	| { type: 'reasoning'; text: string }
	| {
			type: 'tool';
			tool_name: string;
			tool_input: unknown;
			tool_output: string;
			tool_error: boolean;
	  };

export type SubagentProgressEntry =
	| { kind: 'stream'; channel: 'message' | 'thought' | 'plan'; text: string }
	| { kind: 'tool'; title: string; status: string };

export type StreamEvent =
	| { type: 'text_delta'; delta: string }
	| { type: 'reasoning_start' }
	| { type: 'reasoning_delta'; text: string }
	| { type: 'tool_call'; id: string; tool: string; input: unknown }
	| { type: 'tool_result'; id: string; tool: string; output: string; error?: string }
	| { type: 'step_finish'; usage?: TokenUsage }
	| { type: 'subagent_started'; child_session_id: string; purpose: string; agent_name: string }
	| {
			type: 'subagent_progress';
			child_session_id: string;
			progress_kind: string;
			progress_text: string;
	  }
	| {
			type: 'subagent_finished';
			child_session_id: string;
			delegation_status: string;
			summary: string;
	  }
	| {
			type: 'subagent_awaiting_input';
			child_session_id: string;
			kind: string;
			question: string;
			permission_options?: { id: string; kind: string; name: string }[];
	  }
	| {
			type: 'memory_injected';
			memories: {
				id: string;
				content: string;
				kind: string;
				similarity: number;
				effective_weight: number;
			}[];
	  }
	| {
			type: 'memory_updated';
			changes: MemoryUpdate[];
	  }
	| { type: 'error'; message: string; code?: string }
	| { type: 'done' };

export type MemoryUpdate = {
	action: 'create' | 'update' | 'supersede';
	kind: string;
	content: string;
	id?: string;
};

export type ChatItem =
	| { id: string; type: 'user'; text: string; images?: ImageAttachment[]; reveal?: boolean }
	| {
			id: string;
			type: 'assistant';
			text: string;
			pending?: boolean;
			reasoning?: { text: string; pending?: boolean };
			memoryUpdates?: MemoryUpdate[];
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
	  }
	| { id: string; type: 'status'; text: string; usage?: TokenUsage }
	| {
			id: string;
			type: 'memory';
			memories: { id: string; content: string; kind: string; similarity: number; effective_weight: number }[];
	  }
	| { id: string; type: 'error'; text: string }
	| {
			id: string;
			type: 'subagent';
			childSessionId: string;
			purpose: string;
			agentName: string;
			status:
				| 'running'
				| 'completed'
				| 'failed'
				| 'cancelled'
				| 'pending'
				| 'awaiting_user'
				| 'awaiting_permission';
			progress: SubagentProgressEntry[];
			summary?: string;
			pending?: boolean;
			pendingQuestion?: string;
			permissionOptions?: { id: string; kind: string; name: string }[];
	  };
