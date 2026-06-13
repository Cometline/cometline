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

export interface PostMessageRequest {
	text: string;
}

export interface ProviderSettings {
	provider: string;
	baseURL: string;
	apiKey: string;
	selectedModel: string;
	models: string[];
}

export interface SessionListResponse {
	sessions: Session[];
}

export interface TranscriptResponse {
	session_id: string;
	items: TranscriptItem[];
}

export type TranscriptItem =
	| { type: 'user'; text: string }
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
