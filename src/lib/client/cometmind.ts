import type {
	CreateSessionRequest,
	PostMessageRequest,
	Session,
	SessionListResponse,
	StreamEvent,
	TranscriptResponse
} from '$lib/types';

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
	req: PostMessageRequest
): AsyncGenerator<StreamEvent, void, unknown> {
	const res = await fetch(`${BASE_URL}/api/v1/sessions/${id}/message`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req)
	});

	if (!res.ok || !res.body) {
		const text = await res.text();
		throw new Error(`${res.status}: ${text || res.statusText}`);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	let buffer = '';

	try {
		while (true) {
			const { done, value } = await reader.read();
			if (done) break;
			buffer += decoder.decode(value, { stream: true });

			const lines = buffer.split('\n');
			buffer = lines.pop() ?? '';

			for (const line of lines) {
				const trimmed = line.trim();
				if (!trimmed.startsWith('data:')) continue;
				const payload = trimmed.slice(5).trim();
				if (!payload) continue;
				if (payload === '[DONE]') return;
				try {
					yield JSON.parse(payload) as StreamEvent;
				} catch {
					// Ignore malformed SSE payloads.
				}
			}
		}

		const trailing = buffer.trim();
		if (trailing.startsWith('data:')) {
			const payload = trailing.slice(5).trim();
			if (payload && payload !== '[DONE]') {
				try {
					yield JSON.parse(payload) as StreamEvent;
				} catch {
					// Ignore malformed SSE payloads.
				}
			}
		}
	} finally {
		reader.releaseLock();
	}
}

export async function sendMessage(id: string, text: string): Promise<void> {
	for await (const event of streamMessage(id, { text })) {
		if (event.type === 'error') {
			throw new Error(event.message);
		}
	}
}
