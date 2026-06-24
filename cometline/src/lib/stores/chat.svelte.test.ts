import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { StreamEvent } from '$lib/types';
import { getReasoningSegments } from '$lib/conversation/reasoning';
import {
	buildAssistantTimeline,
	buildThinkingAttribution,
	shouldGroupAssistantTimeline
} from '$lib/conversation/thinking-attribution';

const { goto } = vi.hoisted(() => ({ goto: vi.fn() }));

vi.mock('$app/environment', () => ({ browser: true }));
vi.mock('$app/navigation', () => ({ goto }));

vi.mock('$lib/client/cometmind', () => ({
	getSessionMessages: vi.fn(),
	isSessionNotFoundError: vi.fn((err) => err?.code === 'session_not_found'),
	listChildSessions: vi.fn(),
	streamMessage: vi.fn(),
	abortSession: vi.fn()
}));

import { getSessionMessages, listChildSessions, streamMessage } from '$lib/client/cometmind';
import { chatStore } from './chat.svelte';
import { sessionStore } from './session.svelte';
import { startNewChat } from '$lib/actions/new-chat';

async function flushAnimationFrames() {
	await new Promise<void>((resolve) => {
		if (typeof requestAnimationFrame === 'function') {
			requestAnimationFrame(() => resolve());
		} else {
			resolve();
		}
	});
}

async function waitForStore(predicate: () => boolean) {
	await vi.waitFor(async () => {
		await flushAnimationFrames();
		expect(predicate()).toBe(true);
	});
}

async function* eventsOf(...events: StreamEvent[]) {
	for (const event of events) {
		yield event;
	}
}

function mockTranscript(sessionId: string, text: string) {
	return {
		session_id: sessionId,
		items: [{ type: 'user' as const, text }]
	};
}

function hasReasoningText(text: string) {
	return chatStore.items.some(
		(item) =>
			item.type === 'assistant' &&
			getReasoningSegments(item.reasoning).some((segment) => segment.text.includes(text))
	);
}

describe('chatStore session switching', () => {
	beforeEach(() => {
		chatStore.clear();
		sessionStore.setSessions([]);
		goto.mockClear();
		vi.clearAllMocks();
		vi.mocked(listChildSessions).mockResolvedValue({ sessions: [] });
		vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
			cb(0);
			return 0;
		});
	});

	it('discards missing sessions without requesting children', async () => {
		sessionStore.setSessions([
			{
				id: 'missing-session',
				workspace_id: 'workspace-1',
				workspace_path: '/tmp/workspace',
				title: 'Missing',
				model_id: 'model',
				provider_id: 'provider',
				status: 'active',
				token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
				pinned: false,
				created_at: 0,
				updated_at: 0
			}
		]);
		vi.mocked(getSessionMessages).mockRejectedValue({ code: 'session_not_found' });

		chatStore.bindSession('missing-session');
		await chatStore.loadTranscript('missing-session');

		expect(listChildSessions).not.toHaveBeenCalled();
		expect(sessionStore.sessions).toEqual([]);
		expect(chatStore.sessionID).toBe(null);
		expect(goto).toHaveBeenCalledWith('/');
	});

	it('loads session B transcript when switching from session A with partial stream', async () => {
		vi.mocked(streamMessage).mockImplementation(async function* (sessionId) {
			if (sessionId === 'sess-a') {
				yield { type: 'text_delta', delta: 'partial A' };
				await new Promise<void>(() => {});
			}
		});

		vi.mocked(getSessionMessages).mockImplementation(async (sessionId) =>
			mockTranscript(sessionId, `history ${sessionId}`)
		);

		chatStore.bindSession('sess-a');
		void chatStore.send('sess-a', 'question A');
		await vi.waitFor(() =>
			expect(chatStore.items.some((item) => item.type === 'assistant')).toBe(true)
		);

		chatStore.bindSession('sess-b');
		await chatStore.loadTranscript('sess-b');

		expect(chatStore.sessionID).toBe('sess-b');
		expect(chatStore.items).toEqual([
			expect.objectContaining({ type: 'user', text: 'history sess-b' })
		]);
		expect(chatStore.isStreamingFor('sess-b')).toBe(false);
	});

	it('restores session A cache when switching back during stream', async () => {
		let releaseA: (() => void) | undefined;
		const aGate = new Promise<void>((resolve) => {
			releaseA = resolve;
		});

		vi.mocked(streamMessage).mockImplementation(async function* (sessionId) {
			if (sessionId === 'sess-a') {
				yield { type: 'text_delta', delta: 'live A' };
				await aGate;
				yield { type: 'done' };
			}
		});

		vi.mocked(getSessionMessages).mockResolvedValue(mockTranscript('sess-b', 'history B'));

		chatStore.bindSession('sess-a');
		void chatStore.send('sess-a', 'question A');
		await vi.waitFor(() =>
			expect(chatStore.items.some((item) => item.type === 'assistant')).toBe(true)
		);

		const assistantBeforeLeave = chatStore.items.find((item) => item.type === 'assistant');

		chatStore.bindSession('sess-b');
		await chatStore.loadTranscript('sess-b');
		expect(
			chatStore.items.some((item) => item.type === 'user' && item.text === 'history B')
		).toBe(true);

		chatStore.bindSession('sess-a');
		const assistantOnReturn = chatStore.items.find((item) => item.type === 'assistant');
		expect(assistantOnReturn?.id).toBe(assistantBeforeLeave?.id);
		expect(assistantOnReturn?.type === 'assistant' ? assistantOnReturn.text : '').toContain(
			'live A'
		);

		releaseA!();
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(false));
	});

	it('allows concurrent sends in different sessions', async () => {
		vi.mocked(streamMessage).mockImplementation(async function* (sessionId) {
			if (sessionId === 'sess-a') {
				yield* eventsOf({ type: 'text_delta', delta: 'answer A' }, { type: 'done' });
				return;
			}
			if (sessionId === 'sess-b') {
				yield* eventsOf({ type: 'text_delta', delta: 'answer B' }, { type: 'done' });
			}
		});

		chatStore.bindSession('sess-a');
		const sendA = chatStore.send('sess-a', 'question A');
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(true));

		chatStore.bindSession('sess-b');
		const sendB = chatStore.send('sess-b', 'question B');
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-b')).toBe(true));

		await Promise.all([sendA, sendB]);

		expect(chatStore.isStreamingFor('sess-a')).toBe(false);
		expect(chatStore.isStreamingFor('sess-b')).toBe(false);

		chatStore.bindSession('sess-a');
		expect(
			chatStore.items.some(
				(item) => item.type === 'assistant' && item.text.includes('answer A')
			)
		).toBe(true);

		chatStore.bindSession('sess-b');
		expect(
			chatStore.items.some(
				(item) => item.type === 'assistant' && item.text.includes('answer B')
			)
		).toBe(true);
	});

	it('detaches active session without aborting background stream', async () => {
		let releaseA: (() => void) | undefined;
		const aGate = new Promise<void>((resolve) => {
			releaseA = resolve;
		});

		vi.mocked(streamMessage).mockImplementation(async function* (sessionId) {
			if (sessionId === 'sess-a') {
				yield { type: 'text_delta', delta: 'background A' };
				await aGate;
				yield { type: 'done' };
			}
		});

		chatStore.bindSession('sess-a');
		const sendA = chatStore.send('sess-a', 'question A');
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(true));

		chatStore.detachActiveSession();

		expect(chatStore.sessionID).toBe(null);
		expect(chatStore.isStreamingFor('sess-a')).toBe(true);

		releaseA!();
		await sendA;

		chatStore.bindSession('sess-a');
		expect(
			chatStore.items.some(
				(item) => item.type === 'assistant' && item.text.includes('background A')
			)
		).toBe(true);
	});

	it('starts pending first turn in background when new chat is opened before activation', async () => {
		sessionStore.appendSession({
			id: 'sess-a',
			workspace_id: 'workspace-1',
			workspace_path: '/tmp/workspace',
			title: 'Pending',
			model_id: 'model',
			provider_id: 'provider',
			status: 'active',
			token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
			pinned: false,
			created_at: 0,
			updated_at: 0
		});
		sessionStore.queuePendingMessage('sess-a', 'question A', undefined);
		vi.mocked(streamMessage).mockImplementation(async function* (sessionId) {
			if (sessionId === 'sess-a') {
				yield { type: 'text_delta', delta: 'answer A' };
				yield { type: 'done' };
			}
		});

		startNewChat();

		await vi.waitFor(() =>
			expect(streamMessage).toHaveBeenCalledWith(
				'sess-a',
				expect.objectContaining({ text: 'question A' }),
				expect.any(AbortSignal)
			)
		);
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(false));
		expect(sessionStore.hasPendingMessage('sess-a')).toBe(false);

		chatStore.bindSession('sess-a');
		expect(chatStore.items).toEqual([
			expect.objectContaining({ type: 'user', text: 'question A' }),
			expect.objectContaining({ type: 'assistant', text: 'answer A' })
		]);
	});

	it('surfaces an error item when the model fails before any output (e.g. 401)', async () => {
		vi.mocked(streamMessage).mockImplementation(async function* () {
			yield { type: 'error', message: 'cometsdk: openai: authentication failed (HTTP 401)' };
		});

		chatStore.bindSession('sess-a');
		await chatStore.send('sess-a', 'hi');

		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(false));

		const errorItem = chatStore.items.find((item) => item.type === 'error');
		expect(errorItem).toBeDefined();
		if (errorItem?.type !== 'error') return;
		expect(errorItem.text).toContain('API key is invalid or missing');
		// The empty pending assistant must not linger as a blank row.
		expect(chatStore.items.some((item) => item.type === 'assistant' && !item.text.trim())).toBe(
			false
		);
	});

	it('surfaces an error item when streamMessage throws before any output', async () => {
		vi.mocked(streamMessage).mockImplementation(async function* () {
			throw new Error('cometsdk: openai: authentication failed (HTTP 401)');
			yield { type: 'done' };
		});

		chatStore.bindSession('sess-a');
		await chatStore.send('sess-a', 'hi');

		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(false));

		const errorItem = chatStore.items.find((item) => item.type === 'error');
		expect(errorItem).toBeDefined();
		if (errorItem?.type !== 'error') return;
		expect(errorItem.text).toContain('API key is invalid or missing');
	});

	it('stageUserForSession writes to target session when active session differs', () => {
		chatStore.bindSession('sess-a');
		chatStore.stageUserForSession('sess-b', 'message for B');
		chatStore.bindSession('sess-b');
		expect(
			chatStore.items.some((item) => item.type === 'user' && item.text === 'message for B')
		).toBe(true);
		chatStore.bindSession('sess-a');
		expect(
			chatStore.items.some((item) => item.type === 'user' && item.text === 'message for B')
		).toBe(false);
	});

	it('blocks duplicate send in the same session', async () => {
		let releaseA: (() => void) | undefined;
		const aGate = new Promise<void>((resolve) => {
			releaseA = resolve;
		});

		vi.mocked(streamMessage).mockImplementation(async function* () {
			yield { type: 'text_delta', delta: 'working' };
			await aGate;
			yield { type: 'done' };
		});

		chatStore.bindSession('sess-a');
		void chatStore.send('sess-a', 'first');
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(true));

		await expect(chatStore.send('sess-a', 'second')).rejects.toThrow(
			'Session is already streaming'
		);
		expect(vi.mocked(streamMessage)).toHaveBeenCalledTimes(1);

		releaseA!();
	});

	it('does not clobber in-flight cache when loadTranscript returns user-only server data', async () => {
		let releaseA: (() => void) | undefined;
		const aGate = new Promise<void>((resolve) => {
			releaseA = resolve;
		});

		vi.mocked(streamMessage).mockImplementation(async function* (sessionId) {
			if (sessionId === 'sess-a') {
				yield { type: 'reasoning_start' };
				yield { type: 'reasoning_delta', text: 'thinking about jokes' };
				await aGate;
				yield { type: 'done' };
			}
		});

		vi.mocked(getSessionMessages).mockImplementation(async (sessionId) =>
			mockTranscript(sessionId, `server-only ${sessionId}`)
		);

		chatStore.bindSession('sess-a');
		void chatStore.send('sess-a', 'write a joke');
		await waitForStore(() => hasReasoningText('thinking about jokes'));

		chatStore.bindSession('sess-b');
		await chatStore.loadTranscript('sess-b');

		chatStore.bindSession('sess-a');
		await chatStore.loadTranscript('sess-a');

		expect(hasReasoningText('thinking about jokes')).toBe(true);

		releaseA!();
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(false));
	});

	it('preserves partial reasoning when switching back during stream', async () => {
		let releaseA: (() => void) | undefined;
		const aGate = new Promise<void>((resolve) => {
			releaseA = resolve;
		});

		vi.mocked(streamMessage).mockImplementation(async function* (sessionId) {
			if (sessionId === 'sess-a') {
				yield { type: 'reasoning_start' };
				yield { type: 'reasoning_delta', text: 'live reasoning' };
				await aGate;
				yield { type: 'done' };
			}
			if (sessionId === 'sess-b') {
				yield { type: 'text_delta', delta: 'answer B' };
				yield { type: 'done' };
			}
		});

		vi.mocked(getSessionMessages).mockImplementation(async (sessionId) =>
			mockTranscript(sessionId, `history ${sessionId}`)
		);

		chatStore.bindSession('sess-a');
		void chatStore.send('sess-a', 'question A');
		await waitForStore(() => hasReasoningText('live reasoning'));

		chatStore.bindSession('sess-b');
		void chatStore.send('sess-b', 'question B');
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-b')).toBe(true));

		chatStore.bindSession('sess-a');
		expect(hasReasoningText('live reasoning')).toBe(true);

		releaseA!();
		await vi.waitFor(() => expect(chatStore.isStreamingFor('sess-a')).toBe(false));
	});

	it('coalesces historical assistant steps across tool rows on transcript reload', async () => {
		vi.mocked(getSessionMessages).mockResolvedValue({
			session_id: 'sess-a',
			items: [
				{ type: 'user', text: 'inspect main.go' },
				{ type: 'reasoning', text: 'Need to inspect files.' },
				{
					type: 'tool',
					tool_name: 'read_file',
					tool_input: { path: 'main.go' },
					tool_output: 'package main'
				},
				{ type: 'assistant', text: 'The file contains Go code.' }
			]
		});

		chatStore.bindSession('sess-a');
		await chatStore.loadTranscript('sess-a');

		expect(chatStore.items).toHaveLength(3);
		expect(chatStore.items[0]).toMatchObject({ type: 'user', text: 'inspect main.go' });
		expect(chatStore.items[1]).toMatchObject({
			type: 'assistant',
			text: 'The file contains Go code.',
			reasoning: {
				segments: [{ text: 'Need to inspect files.', pending: false }]
			}
		});
		expect(chatStore.items[2]).toMatchObject({
			type: 'tool',
			toolName: 'read_file',
			output: 'package main',
			afterSegment: 0
		});
	});

	it('groups reasoning-less memory and tools under the assistant on reload', async () => {
		// Reasoning-less turns persist as: memory, tool(s), then assistant text.
		// The assistant placeholder must be emitted before its memory/tool rows so
		// the attribution scan groups them instead of leaving loose pills.
		vi.mocked(getSessionMessages).mockResolvedValue({
			session_id: 'sess-a',
			items: [
				{ type: 'user', text: 'run ls and pwd for me' },
				{
					type: 'memory',
					memories: [
						{
							id: 'm1',
							content: 'prefers zsh',
							kind: 'semantic',
							similarity: 0.9,
							effective_weight: 1
						}
					]
				},
				{
					type: 'tool',
					tool_name: 'list_dir',
					tool_input: { path: '.' },
					tool_output: 'a\nb'
				},
				{
					type: 'tool',
					tool_name: 'run_command',
					tool_input: { command: 'pwd' },
					tool_output: '/tmp'
				},
				{ type: 'assistant', text: 'Here are the results.' }
			]
		});

		chatStore.bindSession('sess-a');
		await chatStore.loadTranscript('sess-a');

		const assistant = chatStore.items.find((item) => item.type === 'assistant');
		expect(assistant).toBeTruthy();
		const assistantIndex = chatStore.items.findIndex((item) => item === assistant);
		const memoryIndex = chatStore.items.findIndex((item) => item.type === 'memory');
		const firstToolIndex = chatStore.items.findIndex((item) => item.type === 'tool');

		// Assistant must precede its memory/tool rows so they get grouped.
		expect(assistantIndex).toBeGreaterThanOrEqual(0);
		expect(memoryIndex).toBeGreaterThan(assistantIndex);
		expect(firstToolIndex).toBeGreaterThan(assistantIndex);

		const attribution = buildThinkingAttribution(chatStore.items);
		const timeline = buildAssistantTimeline(assistant!.id, chatStore.items, attribution);
		// memory + 2 tools, all grouped.
		expect(timeline.map((entry) => entry.kind)).toEqual(['memory', 'tool', 'tool']);
		expect(shouldGroupAssistantTimeline(assistant as never, timeline)).toBe(true);
		// No loose pills: every tool/memory is attributed to the assistant.
		expect(attribution.toolIdsInBuffer.size).toBe(2);
		expect(attribution.memoryIdsInBuffer.size).toBe(1);

		// The auto-created assistant placeholder must not collide with the id of
		// the row that triggered it, or the keyed transcript throws
		// `each_key_duplicate`.
		const ids = chatStore.items.map((item) => item.id);
		expect(new Set(ids).size).toBe(ids.length);
	});
});

describe('chatStore clear and subagents', () => {
	beforeEach(() => {
		chatStore.clear();
		sessionStore.setSessions([]);
		vi.clearAllMocks();
		vi.mocked(listChildSessions).mockResolvedValue({ sessions: [] });
		vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
			cb(0);
			return 0;
		});
	});

	it('loadTranscript after clear does not show subagent items', async () => {
		vi.mocked(getSessionMessages).mockResolvedValue({
			session_id: 'sess-a',
			items: []
		});
		vi.mocked(listChildSessions).mockResolvedValue({ sessions: [] });

		chatStore.bindSession('sess-a');
		await chatStore.loadTranscript('sess-a');

		expect(chatStore.items).toEqual([]);
		expect(chatStore.items.some((item) => item.type === 'subagent')).toBe(false);
	});

	it('ignores orphan child sessions when transcript has no delegate tool', async () => {
		vi.mocked(getSessionMessages).mockResolvedValue({
			session_id: 'sess-a',
			items: []
		});
		vi.mocked(listChildSessions).mockResolvedValue({
			sessions: [
				{
					id: 'child-1',
					workspace_id: 'workspace-1',
					workspace_path: '/tmp/workspace',
					title: 'Delegated task',
					model_id: 'model',
					provider_id: 'provider',
					status: 'active',
					token_usage: {
						input_tokens: 0,
						output_tokens: 0,
						cache_read: 0,
						cache_write: 0
					},
					pinned: false,
					parent_session_id: 'sess-a',
					purpose: 'refactor auth',
					delegation_status: 'completed',
					output_summary: 'done',
					created_at: 0,
					updated_at: 0
				}
			]
		});

		chatStore.bindSession('sess-a');
		await chatStore.loadTranscript('sess-a');

		expect(chatStore.items).toEqual([]);
		expect(chatStore.items.some((item) => item.type === 'subagent')).toBe(false);
	});
});
