import {
	abortSession as abortSessionApi,
	compactMemory as compactMemoryApi,
	compactMemoryPreview as compactMemoryPreviewApi,
	clearSession as clearSessionApi,
	createMemory as createMemoryApi,
	createSession as createSessionApi,
	createWorkspace as createWorkspaceApi,
	forkSession as forkSessionApi,
	deleteMemory as deleteMemoryApi,
	deleteSession as deleteSessionApi,
	deleteSkill as deleteSkillApi,
	getSkill as getSkillApi,
	getSkillDraft as getSkillDraftApi,
	updateSkill as updateSkillApi,
	updateSkillDraft as updateSkillDraftApi,
	getMemorySettings as getMemorySettingsApi,
	getSession as getSessionApi,
	getSessionMessages as getSessionMessagesApi,
	listChildSessions as listChildSessionsApi,
	listSkillDrafts as listSkillDraftsApi,
	listMemories as listMemoriesApi,
	lookupModelCatalog as lookupModelCatalogApi,
	listSessions as listSessionsApi,
	listSkills as listSkillsApi,
	listMcpServers as listMcpServersApi,
	listMcpTools as listMcpToolsApi,
	listWorkspaces as listWorkspacesApi,
	deleteWorkspace as deleteWorkspaceApi,
	pruneWorkspaces as pruneWorkspacesApi,
	listWorkspaceFiles as listWorkspaceFilesApi,
	listWorkspaceFileChildren as listWorkspaceFileChildrenApi,
	getWorkspaceGitStatus as getWorkspaceGitStatusApi,
	getWorkspaceGitDiff as getWorkspaceGitDiffApi,
	stageWorkspaceGitPaths as stageWorkspaceGitPathsApi,
	unstageWorkspaceGitPaths as unstageWorkspaceGitPathsApi,
	discardWorkspaceGitPaths as discardWorkspaceGitPathsApi,
	commitWorkspaceGit as commitWorkspaceGitApi,
	readWorkspaceFileContent as readWorkspaceFileContentApi,
	writeWorkspaceFileContent as writeWorkspaceFileContentApi,
	listWikiFiles as listWikiFilesApi,
	listWikiFileChildren as listWikiFileChildrenApi,
	listWikiFileBacklinks as listWikiFileBacklinksApi,
	readWikiFileContent as readWikiFileContentApi,
	writeWikiFileContent as writeWikiFileContentApi,
	patchSession as patchSessionApi,
	putMemorySettings as putMemorySettingsApi,
	runStorageRetention as runStorageRetentionApi,
	runStorageBackup as runStorageBackupApi,
	reconnectMcpServer as reconnectMcpServerApi,
	testMcpServer as testMcpServerApi,
	rejectSkillDraft as rejectSkillDraftApi,
	promoteSkillDraft as promoteSkillDraftApi,
	startMcpOAuth as startMcpOAuthApi,
	searchMemories as searchMemoriesApi,
	syncSkills as syncSkillsApi,
	listJobs as listJobsApi,
	createJob as createJobApi,
	getJob as getJobApi,
	updateJob as updateJobApi,
	deleteJob as deleteJobApi,
	claimJob as claimJobApi,
	listJobEvents as listJobEventsApi,
	releaseJob as releaseJobApi,
	completeJob as completeJobApi,
	archiveJob as archiveJobApi,
	unarchiveJob as unarchiveJobApi,
	unblockJob as unblockJobApi,
	listScheduledJobs as listScheduledJobsApi,
	createScheduledJob as createScheduledJobApi,
	updateScheduledJob as updateScheduledJobApi,
	deleteScheduledJob as deleteScheduledJobApi,
	listInboxMessages as listInboxMessagesApi,
	getInboxSummary as getInboxSummaryApi,
	replyInboxMessage as replyInboxMessageApi,
	dismissInboxMessage as dismissInboxMessageApi,
	listMedia as listMediaApi,
	importMedia as importMediaApi,
	deleteMedia as deleteMediaApi,
	getUsageSummary as getUsageSummaryApi,
	getUsageSeries as getUsageSeriesApi,
	listUsageEvents as listUsageEventsApi
} from '$lib/generated/cometmind-api';
import type {
	CompactMemoryPreviewResponse,
	MemoryCompactionResult,
	CreateMemoryRequest,
	CreateSessionRequest,
	ListMemoriesResponse,
	ListSkillsResponse,
	McpReconnectResponse,
	McpServerStatus,
	McpTestResult,
	McpToolInfo,
	MemoryResource,
	SkillDetailResponse,
	SkillDraft,
	SkillDraftDetailResponse,
	MemorySettings as MemorySettingsWire,
	PostMessageRequest,
	RunStorageRetentionResponse,
	RunStorageBackupResponse,
	Session,
	SessionListResponse,
	StreamEvent,
	SyncSkillsResponse,
	TranscriptResponse,
	UpdateSessionRequest,
	Workspace,
	WorkspaceFileContent,
	WorkspaceGitStatus,
	WorkspaceGitDiff,
	WorkspaceGitMutationResult,
	WorkspaceGitCommitResult,
	JobResource,
	ListJobsResponse,
	JobEventResource,
	CreateJobRequest,
	UpdateJobRequest,
	ScheduledJobResource,
	ListScheduledJobsResponse,
	CreateScheduledJobRequest,
	UpdateScheduledJobRequest,
	InboxMessageResource,
	ListInboxMessagesResponse,
	InboxSummaryResponse,
	MediaResource,
	MediaListResponse,
	UsageSummaryResponse,
	UsageSeriesResponse,
	UsageEventsResponse
} from '$lib/generated/cometmind-api';
import { client } from '$lib/generated/cometmind-api/client.gen';
import { createSSEParser } from '$lib/sse/parser';
import {
	buildJobExecutionPrompt as buildJobExecutionPromptImpl,
	type JobExecutionPromptInput
} from '$lib/jobs/build-job-execution-prompt';

export type {
	CompactMemoryPreviewResponse,
	MemoryCompactionResult,
	CreateMemoryRequest,
	McpServerStatus,
	McpToolInfo,
	MemoryResource,
	RunStorageRetentionResponse,
	RunStorageBackupResponse
} from '$lib/generated/cometmind-api';

export type {
	SkillDetailResponse,
	SkillDraft,
	SkillDraftDetailResponse,
	JobResource,
	JobEventResource,
	CreateJobRequest,
	UpdateJobRequest,
	ScheduledJobResource,
	ListScheduledJobsResponse,
	CreateScheduledJobRequest,
	UpdateScheduledJobRequest,
	InboxMessageResource,
	ListInboxMessagesResponse,
	InboxSummaryResponse,
	MediaResource,
	UsageEventsResponse,
	UsageSeriesResponse,
	UsageSummaryResponse,
	Workspace
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
	task_outcome_limit: number;
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

export class UnexpectedStreamEndError extends Error {
	constructor() {
		super('The response stream ended before CometMind sent a done event.');
		this.name = 'UnexpectedStreamEndError';
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

export function apiErrorMessage(error: unknown, fallback: string): string {
	if (error instanceof Error && error.message) return error.message;
	if (typeof error === 'string' && error.trim()) return error;
	if (error && typeof error === 'object' && 'error_hint' in error) {
		const hint = error.error_hint;
		if (typeof hint === 'string' && hint.trim()) return hint;
	}
	if (error && typeof error === 'object' && 'error' in error) {
		const detail = error.error;
		if (typeof detail === 'string' && detail.trim()) return detail;
		if (
			detail &&
			typeof detail === 'object' &&
			'message' in detail &&
			typeof detail.message === 'string' &&
			detail.message.trim()
		) {
			return detail.message;
		}
	}
	return fallback;
}

export async function runStorageBackup(): Promise<RunStorageBackupResponse> {
	try {
		const { data } = await runStorageBackupApi({ throwOnError: true });
		return data;
	} catch (error) {
		throw new Error(apiErrorMessage(error, 'Backup failed'));
	}
}

export interface WorkspaceFiles {
	files: string[];
	truncated: boolean;
}

export function listWorkspaceFiles(
	workspacePath: string,
	query = '',
	limit = 50,
	options?: { index?: boolean }
): Promise<WorkspaceFiles> {
	return listWorkspaceFilesApi({
		query: {
			workspace_path: workspacePath,
			q: query,
			limit,
			...(options?.index ? { index: true } : {})
		},
		throwOnError: true
	}).then(({ data }) => ({ files: data.files ?? [], truncated: Boolean(data.truncated) }));
}

export function listWorkspaceFileChildren(
	workspacePath: string,
	directory = '',
	limit = 50
): Promise<WorkspaceFiles> {
	return listWorkspaceFileChildrenApi({
		query: { workspace_path: workspacePath, directory, limit },
		throwOnError: true
	}).then(({ data }) => ({ files: data.files ?? [], truncated: Boolean(data.truncated) }));
}

export type GitScope = 'working' | 'staged' | 'all';

export type { WorkspaceGitStatus, WorkspaceGitDiff, WorkspaceGitMutationResult, WorkspaceGitCommitResult };

export function getWorkspaceGitStatus(
	workspacePath: string,
	scope: GitScope = 'working'
): Promise<WorkspaceGitStatus> {
	return getWorkspaceGitStatusApi({
		query: { workspace_path: workspacePath, scope },
		throwOnError: true
	}).then(({ data }) => data);
}

export function getWorkspaceGitDiff(
	workspacePath: string,
	path: string,
	scope: GitScope = 'working'
): Promise<WorkspaceGitDiff> {
	return getWorkspaceGitDiffApi({
		query: { workspace_path: workspacePath, path, scope },
		throwOnError: true
	}).then(({ data }) => data);
}

export async function stageWorkspaceGitPaths(
	workspacePath: string,
	paths: string[]
): Promise<WorkspaceGitMutationResult> {
	const { data } = await stageWorkspaceGitPathsApi({
		body: { workspace_path: workspacePath, paths },
		throwOnError: true
	});
	return data;
}

export async function unstageWorkspaceGitPaths(
	workspacePath: string,
	paths: string[]
): Promise<WorkspaceGitMutationResult> {
	const { data } = await unstageWorkspaceGitPathsApi({
		body: { workspace_path: workspacePath, paths },
		throwOnError: true
	});
	return data;
}

export async function discardWorkspaceGitPaths(
	workspacePath: string,
	paths: string[]
): Promise<WorkspaceGitMutationResult> {
	const { data } = await discardWorkspaceGitPathsApi({
		body: { workspace_path: workspacePath, paths },
		throwOnError: true
	});
	return data;
}

export async function commitWorkspaceGit(
	workspacePath: string,
	message: string
): Promise<WorkspaceGitCommitResult> {
	const { data } = await commitWorkspaceGitApi({
		body: { workspace_path: workspacePath, message },
		throwOnError: true
	});
	return data;
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

export function listWikiFiles(query = '', limit = 50): Promise<WorkspaceFiles> {
	return listWikiFilesApi({
		query: { q: query, limit },
		throwOnError: true
	}).then(({ data }) => ({ files: data.files ?? [], truncated: Boolean(data.truncated) }));
}

export function listWikiFileChildren(directory = '', limit = 50): Promise<WorkspaceFiles> {
	return listWikiFileChildrenApi({
		query: { directory, limit },
		throwOnError: true
	}).then(({ data }) => ({ files: data.files ?? [], truncated: Boolean(data.truncated) }));
}

export function listWikiFileBacklinks(path: string): Promise<string[]> {
	return listWikiFileBacklinksApi({
		query: { path },
		throwOnError: true
	}).then(({ data }) => data.backlinks ?? []);
}

export function readWikiFileContent(path: string): Promise<WorkspaceFileContent> {
	return readWikiFileContentApi({
		query: { path },
		throwOnError: true
	}).then(({ data }) => data as WorkspaceFileContent);
}

export async function writeWikiFileContent(path: string, content: string): Promise<void> {
	await writeWikiFileContentApi({
		body: { path, content },
		throwOnError: true
	});
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

export function getSkill(name: string, workspacePath = ''): Promise<SkillDetailResponse> {
	return getSkillApi({
		path: { name },
		query: skillQuery(workspacePath),
		throwOnError: true
	}).then(({ data }) => data);
}

export function updateSkill(
	name: string,
	content: string,
	workspacePath = ''
): Promise<SkillDetailResponse> {
	return updateSkillApi({
		path: { name },
		query: skillQuery(workspacePath),
		body: { content },
		throwOnError: true
	}).then(({ data }) => data);
}

export function listSkillDrafts(): Promise<SkillDraft[]> {
	return listSkillDraftsApi({ throwOnError: true }).then(({ data }) => data.drafts ?? []);
}

export function getSkillDraft(name: string): Promise<SkillDraftDetailResponse> {
	return getSkillDraftApi({ path: { name }, throwOnError: true }).then(({ data }) => data);
}

export function updateSkillDraft(name: string, content: string): Promise<SkillDraftDetailResponse> {
	return updateSkillDraftApi({
		path: { name },
		body: { content },
		throwOnError: true
	}).then(({ data }) => data);
}

export function promoteSkillDraft(name: string): Promise<void> {
	return promoteSkillDraftApi({ path: { name }, throwOnError: true }).then(() => undefined);
}

export function rejectSkillDraft(name: string): Promise<void> {
	return rejectSkillDraftApi({ path: { name }, throwOnError: true }).then(() => undefined);
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

export async function reconnectMcpServer(serverId: string): Promise<void> {
	try {
		await reconnectMcpServerApi({
			path: { id: serverId },
			throwOnError: true
		});
	} catch (error) {
		throw new Error(apiErrorMessage(error, 'Reconnect failed'));
	}
}

export async function testMcpServer(serverId: string): Promise<McpTestResult> {
	const { data, error } = await testMcpServerApi({
		path: { id: serverId },
		throwOnError: false
	});
	if (data) return data;
	if (error && typeof error === 'object' && 'ok' in error && 'tool_count' in error) {
		return error as McpTestResult;
	}
	throw new Error(apiErrorMessage(error, 'Test failed'));
}

// startMcpOAuth runs the full interactive OAuth flow (discovery, dynamic client
// registration, browser authorization, token exchange) in the CometMind runtime
// and reconnects the server on success. This is a long-running call: it resolves
// only after the user completes the browser round-trip.
export async function startMcpOAuth(serverId: string): Promise<McpReconnectResponse> {
	try {
		const { data } = await startMcpOAuthApi({
			path: { id: serverId },
			throwOnError: true
		});
		return data ?? { ok: true, connected: true };
	} catch (error) {
		throw new Error(apiErrorMessage(error, 'OAuth connect failed'));
	}
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

export function listAllSessions(): Promise<SessionListResponse> {
	return listSessionsApi({
		query: { all: true },
		throwOnError: true
	}).then(({ data }) => data);
}

export function lookupModelCatalog(input: {
	method: string;
	providerId?: string;
	modelIds: string[];
}): Promise<
	Array<{
		model_id: string;
		context: number;
		output: number;
		limit_source: 'catalog' | 'fallback';
		vision: boolean;
		vision_known: boolean;
		input_modalities: Array<'text' | 'image' | 'video' | 'audio' | 'pdf'>;
	}>
> {
	return lookupModelCatalogApi({
		body: {
			method: input.method,
			provider_id: input.providerId,
			model_ids: input.modelIds
		},
		throwOnError: true
	}).then(({ data }) => data.models ?? []);
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

export async function* streamSessionEvents(
	id: string,
	signal?: AbortSignal
): AsyncGenerator<StreamEvent, void, unknown> {
	const res = await fetch(`${BASE_URL}/api/v1/sessions/${encodeURIComponent(id)}/events`, {
		method: 'GET',
		headers: { Accept: 'text/event-stream' },
		cache: 'no-store',
		signal
	});
	yield* readTurnStream(res, signal);
}

/** Stream events that can outlive a request-scoped message stream. */
export async function* streamRuntimeEvents(
	signal?: AbortSignal
): AsyncGenerator<StreamEvent, void, unknown> {
	const res = await fetch(`${BASE_URL}/api/v1/events`, {
		method: 'GET',
		headers: { Accept: 'text/event-stream' },
		cache: 'no-store',
		signal
	});
	if (!res.ok || !res.body) {
		throw new CometMindApiError(res.status, 'event_stream_failed', res.statusText);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	const parser = createSSEParser();
	try {
		while (true) {
			if (signal?.aborted) return;
			const { done, value } = await reader.read();
			if (done) break;
			for (const result of parser.feed(decoder.decode(value, { stream: true }))) {
				if (result && result !== 'done') yield result;
			}
		}
		for (const result of parser.flush()) {
			if (result && result !== 'done') yield result;
		}
	} finally {
		reader.releaseLock();
	}
}

/** Keep a background event subscription alive until the caller stops it. */
export function startRuntimeEventStream(
	onEvent: (event: StreamEvent) => void,
	onReconnect?: () => unknown | Promise<unknown>
): () => void {
	const controller = new AbortController();
	let stopped = false;

	const run = async () => {
		while (!stopped) {
			try {
				await onReconnect?.();
			} catch {
				// The event stream reconnect remains useful even if reconciliation fails.
			}
			if (stopped) return;
			try {
				for await (const event of streamRuntimeEvents(controller.signal)) {
					if (stopped) return;
					onEvent(event);
				}
				if (!stopped) {
					await new Promise((resolve) => setTimeout(resolve, 1000));
				}
			} catch {
				if (stopped) return;
				await new Promise((resolve) => setTimeout(resolve, 1000));
			}
		}
	};

	void run();
	return () => {
		stopped = true;
		controller.abort();
	};
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
	yield* readTurnStream(res, signal);
}

async function* readTurnStream(
	res: Response,
	signal?: AbortSignal
): AsyncGenerator<StreamEvent, void, unknown> {
	if (!res.ok || !res.body) {
		const text = await res.text();
		const parsed = parseErrorBody(text || res.statusText);
		throw new CometMindApiError(res.status, parsed.code, parsed.message);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	const parser = createSSEParser();
	let receivedDone = false;

	const finishStream = async () => {
		try {
			await reader.cancel();
		} catch {
			// Best-effort: the server may already have closed the connection.
		}
	};

	try {
		while (true) {
			if (signal?.aborted) {
				throw new DOMException('The response stream was aborted.', 'AbortError');
			}
			const { done, value } = await reader.read();
			if (done) break;
			const chunk = decoder.decode(value, { stream: true });
			for (const result of parser.feed(chunk)) {
				if (result === 'done') {
					receivedDone = true;
					await finishStream();
					return;
				}
				if (result) {
					yield result;
					if (result.type === 'done') {
						receivedDone = true;
						await finishStream();
						return;
					}
				}
			}
		}

		for (const result of parser.flush()) {
			if (result === 'done') {
				receivedDone = true;
				await finishStream();
				return;
			}
			if (result) {
				yield result;
				if (result.type === 'done') {
					receivedDone = true;
					await finishStream();
					return;
				}
			}
		}

		if (!receivedDone && !signal?.aborted) {
			throw new UnexpectedStreamEndError();
		}
	} finally {
		reader.releaseLock();
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
		task_outcome_limit: 3,
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
		task_outcome_limit: raw.task_outcome_limit ?? def.task_outcome_limit,
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
	return withApiError(
		putMemorySettingsApi({
			body: settings,
			throwOnError: true
		}).then(({ data }) => resolveMemorySettings(data))
	);
}

export interface MemoryReembedPreview {
	active_count: number;
	needs_migration: number;
	current_model: string;
	requested_model: string;
	migration_needed: boolean;
}

export interface MemoryReembedJob {
	id?: string;
	status?: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | string;
	from_model?: string;
	to_provider?: string;
	to_model?: string;
	to_base_url?: string;
	total?: number;
	completed?: number;
	cursor_id?: string;
	error?: string;
	created_at?: number;
	updated_at?: number;
}

async function memoryJSON<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE_URL}${path}`, {
		headers: { 'Content-Type': 'application/json' },
		...init
	});
	if (!res.ok) {
		const text = await res.text();
		const parsed = parseErrorBody(text || res.statusText);
		throw new CometMindApiError(res.status, parsed.code, parsed.message);
	}
	return res.json();
}

export function previewMemoryReembed(
	embedding: MemorySettings['embedding']
): Promise<MemoryReembedPreview> {
	return memoryJSON('/api/v1/memories/reembed-preview', {
		method: 'POST',
		body: JSON.stringify(embedding)
	});
}

export function getMemoryReembedJob(): Promise<MemoryReembedJob> {
	return memoryJSON('/api/v1/memories/reembed-jobs');
}

export function startMemoryReembed(
	embedding: MemorySettings['embedding'],
	force = false
): Promise<MemoryReembedJob> {
	return memoryJSON('/api/v1/memories/reembed-jobs', {
		method: 'POST',
		body: JSON.stringify({ embedding, force })
	});
}

export function cancelMemoryReembed(): Promise<MemoryReembedJob> {
	return memoryJSON('/api/v1/memories/reembed-jobs/current/cancellation', {
		method: 'POST'
	});
}

export function compactMemory(): Promise<MemoryCompactionResult> {
	return compactMemoryApi({ throwOnError: true }).then(({ data }) => data);
}

export function compactMemoryPreview(): Promise<CompactMemoryPreviewResponse> {
	return compactMemoryPreviewApi({ throwOnError: true }).then(({ data }) => data);
}

export type JobListQuery = {
	status?: 'todo' | 'ongoing' | 'done' | 'blocked';
	ready_only?: boolean;
	include_deleted?: boolean;
	include_archived?: boolean;
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

export function archiveJob(id: string): Promise<JobResource> {
	return archiveJobApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
}

export function unarchiveJob(id: string): Promise<JobResource> {
	return unarchiveJobApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
}

export function unblockJob(id: string): Promise<JobResource> {
	return unblockJobApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
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

export function listJobEvents(id: string): Promise<{ events: JobEventResource[] }> {
	return listJobEventsApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
}

export function buildJobExecutionPrompt(job: JobExecutionPromptInput): string {
	return buildJobExecutionPromptImpl(job);
}

export function listScheduledJobs(): Promise<ListScheduledJobsResponse> {
	return listScheduledJobsApi({ throwOnError: true }).then(({ data }) => data);
}

export function createScheduledJob(body: CreateScheduledJobRequest): Promise<ScheduledJobResource> {
	return createScheduledJobApi({ body, throwOnError: true }).then(({ data }) => data);
}

export function updateScheduledJob(
	id: string,
	body: UpdateScheduledJobRequest
): Promise<ScheduledJobResource> {
	return updateScheduledJobApi({ path: { id }, body, throwOnError: true }).then(
		({ data }) => data
	);
}

export function deleteScheduledJob(id: string): Promise<void> {
	return deleteScheduledJobApi({ path: { id }, throwOnError: true }).then(() => undefined);
}

export function listInboxMessages(
	status: 'open' | 'archived' = 'open'
): Promise<ListInboxMessagesResponse> {
	return listInboxMessagesApi({ query: { status }, throwOnError: true }).then(({ data }) => data);
}

export function getInboxSummary(): Promise<InboxSummaryResponse> {
	return getInboxSummaryApi({ throwOnError: true }).then(({ data }) => data);
}

export function replyInboxMessage(id: string, content: string): Promise<InboxMessageResource> {
	return replyInboxMessageApi({
		path: { id },
		body: { content },
		throwOnError: true
	}).then(({ data }) => data);
}

export function dismissInboxMessage(id: string): Promise<InboxMessageResource> {
	return dismissInboxMessageApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
}

export function listMedia(query: {
	workspace_id?: string;
	session_id?: string;
	kind?: 'image' | 'video';
} = {}): Promise<MediaListResponse> {
	return listMediaApi({ query, throwOnError: true }).then(({ data }) => data);
}

export function importMedia(id: string, sessionId: string): Promise<MediaResource> {
	return importMediaApi({
		path: { id },
		body: { session_id: sessionId },
		throwOnError: true
	}).then(({ data }) => data);
}

export function deleteMedia(id: string): Promise<MediaResource> {
	return deleteMediaApi({ path: { id }, throwOnError: true }).then(({ data }) => data);
}

export function getUsageSummary(query: {
	from?: number;
	to?: number;
	workspace_id?: string;
} = {}): Promise<UsageSummaryResponse> {
	return getUsageSummaryApi({ query, throwOnError: true }).then(({ data }) => data);
}

export function getUsageSeries(query: {
	from?: number;
	to?: number;
	workspace_id?: string;
	group_by?: 'model' | 'kind';
	tz_offset_min?: number;
} = {}): Promise<UsageSeriesResponse> {
	return getUsageSeriesApi({ query, throwOnError: true }).then(({ data }) => data);
}

export function listUsageEvents(query: {
	from?: number;
	to?: number;
	workspace_id?: string;
	limit?: number;
	offset?: number;
} = {}): Promise<UsageEventsResponse> {
	return listUsageEventsApi({ query, throwOnError: true }).then(({ data }) => data);
}
