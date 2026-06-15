import type {
	CreateSessionRequest,
	PostMessageRequest,
	Session,
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
