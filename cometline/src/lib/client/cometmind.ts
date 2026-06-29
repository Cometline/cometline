import {
	abortSession as abortSessionApi,
	compactMemory as compactMemoryApi,
	compactMemoryPreview as compactMemoryPreviewApi,
	clearSession as clearSessionApi,
	changeSessionWorkspace as changeSessionWorkspaceApi,
	createMemory as createMemoryApi,
	createSession as createSessionApi,
	createWorkspace as createWorkspaceApi,
	forkSession as forkSessionApi,
	deleteMemory as deleteMemoryApi,
	deleteSession as deleteSessionApi,
	deleteSkill as deleteSkillApi,
	getMemorySettings as getMemorySettingsApi,
	getSession as getSessionApi,
	getSessionMessages as getSessionMessagesApi,
	listChildSessions as listChildSessionsApi,
	listMemories as listMemoriesApi,
	listSessions as listSessionsApi,
	listSkills as listSkillsApi,
	listMcpServers as listMcpServersApi,
	listMcpTools as listMcpToolsApi,
	listWorkspaces as listWorkspacesApi,
	deleteWorkspace as deleteWorkspaceApi,
	pruneWorkspaces as pruneWorkspacesApi,
	listWorkspaceFiles as listWorkspaceFilesApi,
	readWorkspaceFileContent as readWorkspaceFileContentApi,
	writeWorkspaceFileContent as writeWorkspaceFileContentApi,
	patchSession as patchSessionApi,
	putMemorySettings as putMemorySettingsApi,
	runStorageRetention as runStorageRetentionApi,
	reconnectMcpServer as reconnectMcpServerApi,
	startMcpOAuth as startMcpOAuthApi,
	searchMemories as searchMemoriesApi,
	syncSkills as syncSkillsApi,
	testMcpServer as testMcpServerApi,
	listJobs as listJobsApi,
	createJob as createJobApi,
	getJob as getJobApi,
	updateJob as updateJobApi,
	deleteJob as deleteJobApi,
	claimJob as claimJobApi,
	listJobEvents as listJobEventsApi,
	releaseJob as releaseJobApi,
	completeJob as completeJobApi,
	heartbeatJob as heartbeatJobApi,
	getJobSettings as getJobSettingsApi,
	putJobSettings as putJobSettingsApi
} from '$lib/generated/cometmind-api';
import type {
	CompactMemoryPreviewResponse,
	CreateMemoryRequest,
	CreateSessionRequest,
	ListMemoriesResponse,
	ListSkillsResponse,
	McpServerStatus,
	McpTestResult,
	McpToolInfo,
	MemoryResource,
	MemorySettings as MemorySettingsWire,
	PostMessageRequest,
	RunStorageRetentionResponse,
	Session,
	SessionListResponse,
	StreamEvent,
	SyncSkillsResponse,
	TranscriptResponse,
	UpdateSessionRequest,
	Workspace,
	WorkspaceFileContent,
	JobResource,
	ListJobsResponse,
	JobEventResource,
	JobSettings,
	CreateJobRequest,
	UpdateJobRequest
} from '$lib/generated/cometmind-api';
import { client } from '$lib/generated/cometmind-api/client.gen';
import { createSSEParser } from '$lib/sse/parser';
import {
	buildJobExecutionPrompt as buildJobExecutionPromptImpl,
	type JobExecutionPromptInput
} from '$lib/jobs/build-job-execution-prompt';

export type {
	CompactMemoryPreviewResponse,
	CreateMemoryRequest,
	McpServerStatus,
	McpTestResult,
	McpToolInfo,
	MemoryResource,
	RunStorageRetentionResponse
} from '$lib/generated/cometmind-api';

export type {
	JobResource,
	JobEventResource,
	JobSettings,
	CreateJobRequest,
	UpdateJobRequest
} from '$lib/generated/cometmind-api';

export type MemoryLifecycleSettings = {
	decay_half_life_days: number;
	forget_threshold: number;
	usage_boost_factor: number;
	max_usage_boost: number;
	max_memories: number;
	compaction_target_ratio: number;
	compaction_on_extract: boolean;
};

export type MemoryEmbeddingSettings = {
	provider_id: string;
	provider: string;
	model: string;
	base_url: string;
	api_key?: string;
};

export type MemorySettings = {
	enabled: boolean;
	auto_extract: boolean;
	auto_retrieve: boolean;
	max_retrieved: number;
	similarity_threshold: number;
	extraction_model: string;
	lifecycle: MemoryLifecycleSettings;
	embedding: MemoryEmbeddingSettings;
};

const BASE_URL = 'http://127.0.0.1:7700';

client.setConfig({ baseUrl: BASE_URL });

export class CometMindApiError extends Error {
	status: number;
	code: string;

	constructor(status: number, code: string, message: string) {
		super(message);
		this.name = 'CometMindApiError';
		this.status = status;
		this.code = code;
	}
}

function parseErrorBody(raw: string): { code: string; message: string } {
	try {
		const parsed = JSON.parse(raw);
		return {
			code: parsed?.error?.code ?? parsed?.code ?? '',
			message: parsed?.error?.message ?? parsed?.message ?? raw
		};
	} catch {
		return { code: '', message: raw };
	}
}

function normalizeCometMindError(err: unknown): never {
	if (err instanceof CometMindApiError) throw err;
	if (err && typeof err === 'object') {
		const candidate = err as {
			status?: number;
			response?: { status?: number };
			error?:
				| { error?: { code?: string; message?: string }; code?: string; message?: string }
				| string;
			data?: { error?: { code?: string; message?: string }; code?: string; message?: string };
			message?: string;
		};
		const status = candidate.status ?? candidate.response?.status;
		const payload =
			typeof candidate.error === 'string' ? undefined : (candidate.error ?? candidate.data);
		const code = payload?.error?.code ?? payload?.code ?? '';
		const message = payload?.error?.message ?? payload?.message ?? candidate.message ?? '';
		if (status) throw new CometMindApiError(status, code, message || `HTTP ${status}`);
	}
	if (err instanceof Error) {
		const match = err.message.match(/^(\d+):\s*(.*)$/s);
		if (match) {
			const status = Number(match[1]);
			const parsed = parseErrorBody(match[2]);
			throw new CometMindApiError(status, parsed.code, parsed.message);
		}
	}
	throw err;
}

function withApiError<T>(promise: Promise<T>): Promise<T> {
	return promise.catch(normalizeCometMindError);
}

export function isSessionNotFoundError(err: unknown): boolean {
	return (
		err instanceof CometMindApiError && err.status === 404 && err.code === 'session_not_found'
	);
}

function skillQuery(workspacePath: string) {
	return workspacePath ? { workspace_path: workspacePath } : undefined;
}

export function ensureWorkspace(workspacePath: string): Promise<Workspace> {
	return createWorkspaceApi({
		body: { workspace_path: workspacePath },
		throwOnError: true
	}).then(({ data }) => data);
}

export function listWorkspaces(): Promise<Workspace[]> {
	return listWorkspacesApi({ throwOnError: true }).then(({ data }) => data.workspaces);
}

export async function deleteWorkspace(workspacePath: string): Promise<void> {
	await deleteWorkspaceApi({
		query: { workspace_path: workspacePath },
		throwOnError: true
	});
}

export function pruneWorkspaces(): Promise<{ pruned: number }> {
	return pruneWorkspacesApi({ throwOnError: true }).then(({ data }) => data);
}

export function runStorageRetention(): Promise<RunStorageRetentionResponse> {
	return runStorageRetentionApi({ throwOnError: true }).then(({ data }) => data);
}

export interface WorkspaceFiles {
	files: string[];
	truncated: boolean;
}

export function listWorkspaceFiles(
	workspacePath: string,
	query = '',
	limit = 50
): Promise<WorkspaceFiles> {
	return listWorkspaceFilesApi({
		query: { workspace_path: workspacePath, q: query, limit },
		throwOnError: true
	}).then(({ data }) => ({ files: data.files, truncated: Boolean(data.truncated) }));
}

export function readWorkspaceFileContent(
	workspacePath: string,
	path: string
): Promise<WorkspaceFileContent> {
	return readWorkspaceFileContentApi({
		query: { workspace_path: workspacePath, path },
		throwOnError: true
	}).then(({ data }) => data);
}

export async function writeWorkspaceFileContent(
	workspacePath: string,
	path: string,
	content: string
): Promise<void> {
	await writeWorkspaceFileContentApi({
		body: { workspace_path: workspacePath, path, content },
		throwOnError: true
	});
}

export function changeSessionWorkspace(sessionId: string, workspacePath: string): Promise<Session> {
	return changeSessionWorkspaceApi({
		path: { id: sessionId },
		body: { workspace_path: workspacePath },
		throwOnError: true
	}).then(({ data }) => data);
}

export function forkSession(sessionId: string, workspacePath: string): Promise<Session> {
	return forkSessionApi({
		path: { id: sessionId },
		body: { workspace_path: workspacePath },
		throwOnError: true
	}).then(({ data }) => data);
}

export async function clearSession(sessionId: string): Promise<void> {
	await clearSessionApi({
		path: { id: sessionId },
		throwOnError: true
	});
}

export function listSkills(workspacePath = ''): Promise<ListSkillsResponse> {
	return listSkillsApi({
		query: skillQuery(workspacePath),
		throwOnError: true
	}).then(({ data }) => data);
}

export function syncSkills(workspacePath = ''): Promise<SyncSkillsResponse> {
	return syncSkillsApi({
		query: skillQuery(workspacePath),
		throwOnError: true
	}).then(({ data }) => data);
}

export async function listMcpServers(): Promise<McpServerStatus[]> {
	const { data } = await listMcpServersApi({ throwOnError: true });
	return data.servers ?? [];
}

export async function listMcpTools(): Promise<McpToolInfo[]> {
	const { data } = await listMcpToolsApi({ throwOnError: true });
	return data.tools ?? [];
}

export async function testMcpServer(serverId: string): Promise<McpTestResult> {
	const { data } = await testMcpServerApi({
		path: { id: serverId },
		throwOnError: true
	});
	return data;
}

export async function reconnectMcpServer(serverId: string): Promise<void> {
	await reconnectMcpServerApi({
		path: { id: serverId },
		throwOnError: true
	});
}

// startMcpOAuth runs the full interactive OAuth flow (discovery, dynamic client
// registration, browser authorization, token exchange) in the CometMind runtime
// and reconnects the server on success. This is a long-running call: it resolves
// only after the user completes the browser round-trip.
export async function startMcpOAuth(serverId: string): Promise<void> {
	await startMcpOAuthApi({
		path: { id: serverId },
		throwOnError: true
	});
}

export async function deleteSkill(name: string, workspacePath = ''): Promise<void> {
	await deleteSkillApi({
		path: { name },
		query: skillQuery(workspacePath),
		throwOnError: true
	});
}

export async function exportSkill(name: string, workspacePath = ''): Promise<Blob> {
	const params = workspacePath
		? `?${new URLSearchParams({ workspace_path: workspacePath })}`
		: '';
	const res = await fetch(
		`${BASE_URL}/api/v1/skills/${encodeURIComponent(name)}/archive${params}`
	);
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	return res.blob();
}

export function createSession(req: CreateSessionRequest): Promise<Session> {
	return createSessionApi({
		body: req,
		throwOnError: true
	}).then(({ data }) => data);
}

export function listSessions(workspacePath: string): Promise<SessionListResponse> {
	return listSessionsApi({
		query: { workspace_path: workspacePath },
		throwOnError: true
	}).then(({ data }) => data);
}

export function listAllSessions(): Promise<SessionListResponse> {
	return listSessionsApi({
		query: { all: true },
		throwOnError: true
	}).then(({ data }) => data);
}

export function getSession(id: string): Promise<Session> {
	return withApiError(
		getSessionApi({
			path: { id },
			throwOnError: true
		}).then(({ data }) => data)
	);
}

export function updateSession(id: string, req: UpdateSessionRequest): Promise<Session> {
	return patchSessionApi({
		path: { id },
		body: req,
		throwOnError: true
	}).then(({ data }) => data);
}

export function listChildSessions(id: string): Promise<SessionListResponse> {
	return withApiError(
		listChildSessionsApi({
			path: { id },
			throwOnError: true
		}).then(({ data }) => data)
	);
}

export function getSessionMessages(id: string): Promise<TranscriptResponse> {
	return withApiError(
		getSessionMessagesApi({
			path: { id },
			throwOnError: true
		}).then(({ data }) => data)
	);
}

export async function deleteSession(id: string): Promise<void> {
	await deleteSessionApi({
		path: { id },
		throwOnError: true
	});
}

export function abortSession(id: string): Promise<{ status: string }> {
	return abortSessionApi({
		path: { id },
		throwOnError: true
	}).then(({ data }) => data);
}

export async function* streamMessage(
	id: string,
	req: PostMessageRequest,
	signal?: AbortSignal
): AsyncGenerator<StreamEvent, void, unknown> {
	yield* streamSse(`/api/v1/sessions/${id}/messages`, req, signal);
}

async function* streamSse(
	path: string,
	body: unknown,
	signal?: AbortSignal
): AsyncGenerator<StreamEvent, void, unknown> {
	const res = await fetch(`${BASE_URL}${path}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body),
		signal
	});

	if (!res.ok || !res.body) {
		const text = await res.text();
		const parsed = parseErrorBody(text || res.statusText);
		throw new CometMindApiError(res.status, parsed.code, parsed.message);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	const parser = createSSEParser();

	const finishStream = async () => {
		try {
			await reader.cancel();
		} catch {
			// Best-effort: the server may already have closed the connection.
		}
	};

	try {
		while (true) {
			if (signal?.aborted) return;
			const { done, value } = await reader.read();
			if (done) break;
			const chunk = decoder.decode(value, { stream: true });
			for (const result of parser.feed(chunk)) {
				if (result === 'done') {
					await finishStream();
					return;
				}
				if (result) {
					yield result;
					if (result.type === 'done') {
						await finishStream();
						return;
					}
				}
			}
		}

		for (const result of parser.flush()) {
			if (result === 'done') {
				await finishStream();
				return;
			}
			if (result) {
				yield result;
				if (result.type === 'done') {
					await finishStream();
					return;
				}
			}
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

export function listMemories(): Promise<ListMemoriesResponse> {
	return listMemoriesApi({ throwOnError: true }).then(({ data }) => data);
}

export function createMemory(body: CreateMemoryRequest): Promise<MemoryResource> {
	return createMemoryApi({
		body,
		throwOnError: true
	}).then(({ data }) => data);
}

export function deleteMemory(id: string): Promise<void> {
	return deleteMemoryApi({
		path: { id },
		throwOnError: true
	}).then(() => undefined);
}

export function searchMemories(query: string, limit = 10): Promise<ListMemoriesResponse> {
	return searchMemoriesApi({
		body: { query, limit },
		throwOnError: true
	}).then(({ data }) => data);
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

function resolveMemorySettings(raw: MemorySettingsWire): MemorySettings {
	const def = defaultMemorySettings();
	const lifecycle = raw.lifecycle ?? {};
	const embedding = raw.embedding ?? {};
	return {
		enabled: raw.enabled ?? def.enabled,
		auto_extract: raw.auto_extract ?? def.auto_extract,
		auto_retrieve: raw.auto_retrieve ?? def.auto_retrieve,
		max_retrieved: raw.max_retrieved ?? def.max_retrieved,
		similarity_threshold: raw.similarity_threshold ?? def.similarity_threshold,
		extraction_model: raw.extraction_model ?? def.extraction_model,
		lifecycle: {
			decay_half_life_days:
				lifecycle.decay_half_life_days ?? def.lifecycle.decay_half_life_days,
			forget_threshold: lifecycle.forget_threshold ?? def.lifecycle.forget_threshold,
			usage_boost_factor: lifecycle.usage_boost_factor ?? def.lifecycle.usage_boost_factor,
			max_usage_boost: lifecycle.max_usage_boost ?? def.lifecycle.max_usage_boost,
			max_memories: lifecycle.max_memories ?? def.lifecycle.max_memories,
			compaction_target_ratio:
				lifecycle.compaction_target_ratio ?? def.lifecycle.compaction_target_ratio,
			compaction_on_extract:
				lifecycle.compaction_on_extract ?? def.lifecycle.compaction_on_extract
		},
		embedding: {
			provider_id: embedding.provider_id ?? def.embedding.provider_id,
			provider: embedding.provider ?? def.embedding.provider,
			model: embedding.model ?? def.embedding.model,
			base_url: embedding.base_url ?? def.embedding.base_url,
			api_key: embedding.api_key ?? def.embedding.api_key
		}
	};
}

export function getMemorySettings(): Promise<MemorySettings> {
	return getMemorySettingsApi({ throwOnError: true }).then(({ data }) =>
		resolveMemorySettings(data)
	);
}

export function putMemorySettings(settings: MemorySettings): Promise<MemorySettings> {
	return putMemorySettingsApi({
		body: settings,
		throwOnError: true
	}).then(({ data }) => resolveMemorySettings(data));
}

export interface PurgeArchivedMemoryResponse {
	status: string;
	memories_purged: number;
	memory_events_purged: number;
}

export async function purgeArchivedMemory(
	olderThanDays: number
): Promise<PurgeArchivedMemoryResponse> {
	const res = await fetch(`${BASE_URL}/api/v1/memories/purge-runs`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ older_than_days: olderThanDays })
	});
	if (!res.ok) {
		const text = await res.text();
		const parsed = parseErrorBody(text || res.statusText);
		throw new CometMindApiError(res.status, parsed.code, parsed.message);
	}
	return res.json();
}

export function compactMemory(): Promise<{ status: string }> {
	return compactMemoryApi({ throwOnError: true }).then(({ data }) => data);
}

export function compactMemoryPreview(): Promise<CompactMemoryPreviewResponse> {
	return compactMemoryPreviewApi({ throwOnError: true }).then(({ data }) => data);
}

export type JobListQuery = {
	status?: 'todo' | 'ongoing' | 'done';
	ready_only?: boolean;
	include_deleted?: boolean;
};

export function listJobs(query: JobListQuery = {}): Promise<ListJobsResponse> {
	return listJobsApi({ query, throwOnError: true }).then(({ data }) => data);
}

export function createJob(body: CreateJobRequest): Promise<JobResource> {
	return createJobApi({ body, throwOnError: true }).then(({ data }) => data);
}

export function getJob(id: string): Promise<JobResource> {
	return getJobApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
}

export function updateJob(id: string, body: UpdateJobRequest): Promise<JobResource> {
	return updateJobApi({ path: { id }, body, throwOnError: true }).then(({ data }) => data);
}

export function deleteJob(id: string): Promise<void> {
	return deleteJobApi({ path: { id }, throwOnError: true }).then(() => undefined);
}

export function claimJob(id: string, sessionId: string): Promise<JobResource> {
	return claimJobApi({
		path: { id },
		body: { session_id: sessionId },
		throwOnError: true
	}).then(({ data }) => data);
}

export function releaseJob(id: string, sessionId: string, reason?: string): Promise<JobResource> {
	return releaseJobApi({
		path: { id },
		body: { session_id: sessionId, reason },
		throwOnError: true
	}).then(({ data }) => data);
}

export function completeJob(
	id: string,
	sessionId: string,
	progress?: string
): Promise<JobResource> {
	return completeJobApi({
		path: { id },
		body: { session_id: sessionId, progress },
		throwOnError: true
	}).then(({ data }) => data);
}

export function heartbeatJob(id: string, sessionId: string): Promise<void> {
	return heartbeatJobApi({
		path: { id },
		body: { session_id: sessionId },
		throwOnError: true
	}).then(() => undefined);
}

export function listJobEvents(id: string): Promise<{ events: JobEventResource[] }> {
	return listJobEventsApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
}

export function getJobSettings(): Promise<JobSettings> {
	return getJobSettingsApi({ throwOnError: true }).then(({ data }) => data);
}

export function putJobSettings(settings: JobSettings): Promise<JobSettings> {
	return putJobSettingsApi({ body: settings, throwOnError: true }).then(({ data }) => data);
}

export function buildJobExecutionPrompt(job: JobExecutionPromptInput): string {
	return buildJobExecutionPromptImpl(job);
}
