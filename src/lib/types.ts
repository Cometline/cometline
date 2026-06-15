export interface Session {
	id: string;
	workspace_id: string;
	workspace_path: string;
	title: string;
	model_id: string;
	provider_id: string;
	status: 'active' | 'archived';
	token_usage: TokenUsage;
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

export interface AppearanceSettings {
	heroComposer: HeroComposerAppearance;
}

export type ShortcutAction =
	| 'toggleSidebar'
	| 'openSettings'
	| 'newChat'
	| 'stopResponse'
	| 'sendMessage'
	| 'closeSettings'
	| 'focusSearch'
	| 'previousSession'
	| 'nextSession';

export interface ShortcutBinding {
	key: string;
	command?: boolean;
	ctrl?: boolean;
	meta?: boolean;
	alt?: boolean;
	shift?: boolean;
}

export type KeyboardShortcuts = Partial<Record<ShortcutAction, ShortcutBinding>>;

export interface ProviderSettings {
	providers: ProviderConfig[];
	activeProviderId: string;
	appearance: AppearanceSettings;
	shortcuts: KeyboardShortcuts;
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

export type StreamEvent =
	| { type: 'text_delta'; delta: string }
	| { type: 'reasoning_start' }
	| { type: 'reasoning_delta'; text: string }
	| { type: 'tool_call'; id: string; tool: string; input: unknown }
	| { type: 'tool_result'; id: string; tool: string; output: string; error?: string }
	| { type: 'step_finish'; usage?: TokenUsage }
	| { type: 'error'; message: string; code?: string }
	| { type: 'done' };

export type ChatItem =
	| { id: string; type: 'user'; text: string; images?: ImageAttachment[]; reveal?: boolean }
	| {
			id: string;
			type: 'assistant';
			text: string;
			pending?: boolean;
			reasoning?: { text: string; pending?: boolean };
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
	| { id: string; type: 'error'; text: string };
