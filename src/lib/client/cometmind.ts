import type {
	CreateSessionRequest,
	PostMessageRequest,
	Session,
	SkillListResponse,
	SkillSyncResponse,
	SessionListResponse,
	StreamEvent,
	TranscriptResponse,
	UpdateSessionRequest,
	Workspace
} from '$lib/types';
import { createSSEParser } from '$lib/sse/parser';

const BASE_URL = 'http://127.0.0.1:7700';

async function api<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE_URL}${path}`, {
		...init,
		headers: {
			'Content-Type': 'application/json',
			...init?.headers
		}
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	return res.json() as Promise<T>;
}

export function ensureWorkspace(workspacePath: string): Promise<Workspace> {
	return api<Workspace>('/api/v1/workspaces', {
		method: 'POST',
		body: JSON.stringify({ workspace_path: workspacePath })
	});
}

export function listSkills(workspacePath = ''): Promise<SkillListResponse> {
	const params = workspacePath ? `?${new URLSearchParams({ workspace_path: workspacePath })}` : '';
	return api<SkillListResponse>(`/api/v1/skills${params}`);
}

export function syncSkills(workspacePath = ''): Promise<SkillSyncResponse> {
	const params = workspacePath ? `?${new URLSearchParams({ workspace_path: workspacePath })}` : '';
	return api<SkillSyncResponse>(`/api/v1/skills/sync${params}`, { method: 'POST' });
}

export function createSession(req: CreateSessionRequest): Promise<Session> {
	return api<Session>('/api/v1/sessions', {
		method: 'POST',
		body: JSON.stringify(req)
	});
}

export function listSessions(workspacePath: string): Promise<SessionListResponse> {
	const params = new URLSearchParams({ workspace_path: workspacePath });
	return api<SessionListResponse>(`/api/v1/sessions?${params.toString()}`);
}

export function getSession(id: string): Promise<Session> {
	return api<Session>(`/api/v1/sessions/${id}`);
}

export function updateSession(id: string, req: UpdateSessionRequest): Promise<Session> {
	return api<Session>(`/api/v1/sessions/${id}`, {
		method: 'PATCH',
		body: JSON.stringify(req)
	});
}

export function listChildSessions(id: string): Promise<SessionListResponse> {
	return api<SessionListResponse>(`/api/v1/sessions/${id}/children`);
}

export function getSessionMessages(id: string): Promise<TranscriptResponse> {
	return api<TranscriptResponse>(`/api/v1/sessions/${id}/messages`);
}

export async function deleteSession(id: string): Promise<void> {
	const res = await fetch(`${BASE_URL}/api/v1/sessions/${id}`, { method: 'DELETE' });
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
}

export function abortSession(id: string): Promise<{ status: string }> {
	return api<{ status: string }>(`/api/v1/sessions/${id}/abort`, { method: 'POST' });
}

export interface RespondToSubagentRequest {
	text?: string;
	permission_option_id?: string;
}

export async function* respondToSubagent(
	childId: string,
	req: RespondToSubagentRequest,
	signal?: AbortSignal
): AsyncGenerator<StreamEvent, void, unknown> {
	const res = await fetch(`${BASE_URL}/api/v1/sessions/${childId}/respond`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
		signal
	});

	if (!res.ok || !res.body) {
		const text = await res.text();
		throw new Error(`${res.status}: ${text || res.statusText}`);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	const parser = createSSEParser();

	try {
		while (true) {
			if (signal?.aborted) return;
			const { done, value } = await reader.read();
			if (done) break;
			const chunk = decoder.decode(value, { stream: true });
			for (const result of parser.feed(chunk)) {
				if (result === 'done') return;
				if (result) yield result;
			}
		}

		for (const result of parser.flush()) {
			if (result === 'done') return;
			if (result) yield result;
		}
	} catch (err) {
		if (signal?.aborted || (err instanceof DOMException && err.name === 'AbortError')) return;
		throw err;
	} finally {
		reader.releaseLock();
	}
}

export async function* streamMessage(
	id: string,
	req: PostMessageRequest,
	signal?: AbortSignal
): AsyncGenerator<StreamEvent, void, unknown> {
	const res = await fetch(`${BASE_URL}/api/v1/sessions/${id}/message`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
		signal
	});

	if (!res.ok || !res.body) {
		const text = await res.text();
		throw new Error(`${res.status}: ${text || res.statusText}`);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	const parser = createSSEParser();

	try {
		while (true) {
			if (signal?.aborted) return;
			const { done, value } = await reader.read();
			if (done) break;
			const chunk = decoder.decode(value, { stream: true });
			for (const result of parser.feed(chunk)) {
				if (result === 'done') return;
				if (result) yield result;
			}
		}

		for (const result of parser.flush()) {
			if (result === 'done') return;
			if (result) yield result;
		}
	} catch (err) {
		if (signal?.aborted || (err instanceof DOMException && err.name === 'AbortError')) return;
		throw err;
	} finally {
		reader.releaseLock();
	}
}

export async function sendMessage(id: string, req: PostMessageRequest | string): Promise<void> {
	const body = typeof req === 'string' ? { text: req } : req;
	for await (const event of streamMessage(id, body)) {
		if (event.type === 'error') {
			throw new Error(event.message);
		}
	}
}

export interface MemoryResource {
	id: string;
	scope: string;
	kind: string;
	content: string;
	source: string;
	base_weight: number;
	effective_weight: number;
	access_count: number;
	pinned: boolean;
	last_accessed_at?: number;
	created_at: number;
	updated_at: number;
	similarity?: number;
}

export interface MemorySettings {
	enabled: boolean;
	auto_extract: boolean;
	auto_retrieve: boolean;
	max_retrieved: number;
	similarity_threshold: number;
	extraction_model: string;
	lifecycle: {
		decay_half_life_days: number;
		forget_threshold: number;
		usage_boost_factor: number;
		max_usage_boost: number;
		max_memories: number;
		compaction_target_ratio: number;
		compaction_on_extract: boolean;
	};
	embedding: {
		provider_id: string;
		provider: string;
		model: string;
		base_url: string;
		api_key?: string;
	};
}

export function listMemories(): Promise<{ memories: MemoryResource[] }> {
	return api('/api/v1/memories');
}

export function createMemory(body: {
	content: string;
	kind?: string;
	pinned?: boolean;
	base_weight?: number;
}): Promise<MemoryResource> {
	return api<MemoryResource>('/api/v1/memories', {
		method: 'POST',
		body: JSON.stringify(body)
	});
}

export function deleteMemory(id: string): Promise<void> {
	return fetch(`${BASE_URL}/api/v1/memories/${id}`, { method: 'DELETE' }).then((res) => {
		if (!res.ok) {
			return res.text().then((body) => {
				throw new Error(`${res.status}: ${body || res.statusText}`);
			});
		}
	});
}

export function searchMemories(query: string, limit = 10): Promise<{ memories: MemoryResource[] }> {
	return api('/api/v1/memories/search', {
		method: 'POST',
		body: JSON.stringify({ query, limit })
	});
}

export function defaultMemorySettings(): MemorySettings {
	return {
		enabled: true,
		auto_extract: true,
		auto_retrieve: true,
		max_retrieved: 5,
		similarity_threshold: 0.5,
		extraction_model: '',
		lifecycle: {
			decay_half_life_days: 30,
			forget_threshold: 0.1,
			usage_boost_factor: 0.15,
			max_usage_boost: 2,
			max_memories: 500,
			compaction_target_ratio: 0.8,
			compaction_on_extract: true
		},
		embedding: {
			provider_id: '',
			provider: '',
			model: '',
			base_url: '',
			api_key: ''
		}
	};
}

function normalizeMemorySettings(raw: Record<string, unknown>): MemorySettings {
	const def = defaultMemorySettings();
	const lifecycle = (raw.lifecycle ?? raw.Lifecycle ?? {}) as Record<string, unknown>;
	const embedding = (raw.embedding ?? raw.Embedding ?? {}) as Record<string, unknown>;
	return {
		enabled: Boolean(raw.enabled ?? raw.Enabled ?? def.enabled),
		auto_extract: Boolean(raw.auto_extract ?? raw.AutoExtract ?? def.auto_extract),
		auto_retrieve: Boolean(raw.auto_retrieve ?? raw.AutoRetrieve ?? def.auto_retrieve),
		max_retrieved: Number(raw.max_retrieved ?? raw.MaxRetrieved ?? def.max_retrieved),
		similarity_threshold: Number(
			raw.similarity_threshold ?? raw.SimilarityThreshold ?? def.similarity_threshold
		),
		extraction_model: String(raw.extraction_model ?? raw.ExtractionModel ?? def.extraction_model),
		lifecycle: {
			decay_half_life_days: Number(
				lifecycle.decay_half_life_days ??
					lifecycle.DecayHalfLifeDays ??
					def.lifecycle.decay_half_life_days
			),
			forget_threshold: Number(
				lifecycle.forget_threshold ?? lifecycle.ForgetThreshold ?? def.lifecycle.forget_threshold
			),
			usage_boost_factor: Number(
				lifecycle.usage_boost_factor ??
					lifecycle.UsageBoostFactor ??
					def.lifecycle.usage_boost_factor
			),
			max_usage_boost: Number(
				lifecycle.max_usage_boost ?? lifecycle.MaxUsageBoost ?? def.lifecycle.max_usage_boost
			),
			max_memories: Number(
				lifecycle.max_memories ?? lifecycle.MaxMemories ?? def.lifecycle.max_memories
			),
			compaction_target_ratio: Number(
				lifecycle.compaction_target_ratio ??
					lifecycle.CompactionTargetRatio ??
					def.lifecycle.compaction_target_ratio
			),
			compaction_on_extract: Boolean(
				lifecycle.compaction_on_extract ??
					lifecycle.CompactionOnExtract ??
					def.lifecycle.compaction_on_extract
			)
		},
		embedding: {
			provider_id: String(
				embedding.provider_id ?? embedding.ProviderID ?? def.embedding.provider_id
			),
			provider: String(embedding.provider ?? embedding.Provider ?? def.embedding.provider),
			model: String(embedding.model ?? embedding.Model ?? def.embedding.model),
			base_url: String(embedding.base_url ?? embedding.BaseURL ?? def.embedding.base_url),
			api_key: String(embedding.api_key ?? embedding.APIKey ?? def.embedding.api_key ?? '')
		}
	};
}

export function getMemorySettings(): Promise<MemorySettings> {
	return api<Record<string, unknown>>('/api/v1/memory/settings').then(normalizeMemorySettings);
}

export function putMemorySettings(settings: MemorySettings): Promise<MemorySettings> {
	return api<Record<string, unknown>>('/api/v1/memory/settings', {
		method: 'PUT',
		body: JSON.stringify(settings)
	}).then(normalizeMemorySettings);
}

export function compactMemory(): Promise<{ status: string }> {
	return api('/api/v1/memory/compact', { method: 'POST' });
}

export function compactMemoryPreview(): Promise<{
	to_forget: MemoryResource[];
	to_merge: MemoryResource[][];
	active: number;
	max_memories: number;
}> {
	return api('/api/v1/memory/compact/preview');
}
