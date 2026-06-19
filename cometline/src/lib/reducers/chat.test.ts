import { describe, expect, it } from 'vitest';
import { initChatState, reduceChatState } from './chat';
import type { StreamEvent } from '$lib/types';

describe('reduceChatState', () => {
	it('returns a new state without mutating the input', () => {
		const before = initChatState();
		const after = reduceChatState(before, { type: 'text_delta', delta: 'hi' });
		expect(after).not.toBe(before);
		expect(before.items).toHaveLength(0);
		expect(after.items).toHaveLength(1);
	});

	it('accumulates text deltas into one assistant item', () => {
		let state = initChatState();
		state = reduceChatState(state, { type: 'text_delta', delta: 'Hello' });
		state = reduceChatState(state, { type: 'text_delta', delta: ' world' });
		expect(state.items).toHaveLength(1);
		const assistant = state.items[0];
		expect(assistant.type).toBe('assistant');
		if (assistant.type !== 'assistant') return;
		expect(assistant.text).toBe('Hello world');
		expect(state.assistant?.text).toBe('Hello world');
	});

	it('reuses unchanged item references on streaming deltas', () => {
		const user = {
			id: 'user-1',
			type: 'user' as const,
			text: 'question'
		};
		let state = initChatState();
		state = reduceChatState(state, { type: 'text_delta', delta: 'Hi' });
		state = { ...state, items: [user, ...state.items] };
		state = reduceChatState(state, { type: 'text_delta', delta: ' there' });
		expect(state.items[0]).toBe(user);
	});

	it('attaches reasoning to the assistant bubble', () => {
		let state = initChatState();
		state = reduceChatState(state, { type: 'reasoning_start' });
		state = reduceChatState(state, { type: 'reasoning_delta', text: 'think' });
		expect(state.items).toHaveLength(1);
		const assistant = state.items[0];
		expect(assistant.type).toBe('assistant');
		if (assistant.type !== 'assistant') return;
		expect(assistant.reasoning?.text).toBe('think');
		expect(assistant.reasoning?.pending).toBe(true);
	});

	it('flushes reasoning when text begins', () => {
		let state = initChatState();
		state = reduceChatState(state, { type: 'reasoning_delta', text: 'plan: ' });
		state = reduceChatState(state, { type: 'text_delta', delta: 'answer' });
		const assistant = state.items[0];
		expect(assistant.type).toBe('assistant');
		if (assistant.type !== 'assistant') return;
		expect(assistant.text).toBe('answer');
		expect(assistant.reasoning?.text).toBe('plan: ');
		expect(assistant.reasoning?.pending).toBe(false);
	});

	it('creates tool items and updates them on result', () => {
		let state = initChatState();
		state = reduceChatState(state, {
			type: 'tool_call',
			id: 'tc-1',
			tool: 'read_file',
			input: { path: 'README.md' }
		});
		expect(state.items).toHaveLength(1);
		const tool = state.items[0];
		expect(tool.type).toBe('tool');
		if (tool.type !== 'tool') return;
		expect(tool.toolId).toBe('tc-1');
		expect(tool.pending).toBe(true);
		expect(tool.startedAt).toBeTypeOf('number');

		state = reduceChatState(state, {
			type: 'tool_result',
			id: 'tc-1',
			tool: 'read_file',
			output: 'ok'
		});
		const updated = state.items[0];
		expect(updated.type).toBe('tool');
		if (updated.type !== 'tool') return;
		expect(updated.output).toBe('ok');
		expect(updated.pending).toBe(false);
		expect(updated.startedAt).toBeTypeOf('number');
		expect(updated.durationMs).toBeTypeOf('number');
	});

	it('settles reasoning on step_finish', () => {
		let state = initChatState();
		state = reduceChatState(state, { type: 'reasoning_delta', text: 'done' });
		state = reduceChatState(state, {
			type: 'step_finish',
			usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 }
		});
		const assistant = state.items[0];
		expect(assistant.type).toBe('assistant');
		if (assistant.type !== 'assistant') return;
		expect(assistant.reasoning?.pending).toBe(false);
		expect(assistant.pending).toBe(false);
	});

	it('removes empty assistants on done', () => {
		let state = initChatState();
		state = reduceChatState(state, { type: 'reasoning_start' });
		state = reduceChatState(state, { type: 'done' });
		expect(state.items).toHaveLength(0);
		expect(state.assistant).toBeNull();
	});

	it('appends an error item on error events', () => {
		let state = initChatState();
		state = reduceChatState(state, { type: 'text_delta', delta: 'partial' });
		state = reduceChatState(state, { type: 'error', message: 'something broke' });
		expect(state.items).toHaveLength(2);
		expect(state.items[0].type).toBe('assistant');
		expect(state.items[1].type).toBe('error');
		if (state.items[1].type !== 'error') return;
		expect(state.items[1].text).toBe('something broke');
		expect(state.error).toBe('something broke');
		expect(state.assistant).toBeNull();
	});

	it('rewrites provider header-timeout errors into a readable message', () => {
		let state = initChatState();
		state = reduceChatState(state, {
			type: 'error',
			message:
				'openai: http: Post "https://api.example.com/v1/chat/completions": context deadline exceeded (Client.Timeout exceeded while awaiting headers)'
		});
		expect(state.items).toHaveLength(1);
		expect(state.items[0].type).toBe('error');
		if (state.items[0].type !== 'error') return;
		expect(state.items[0].text).toContain('did not start responding');
		expect(state.items[0].text).not.toContain('Client.Timeout');
	});

	it('rewrites 401 auth errors into a settings hint', () => {
		let state = initChatState();
		state = reduceChatState(state, {
			type: 'error',
			message: 'cometsdk: openai: authentication failed (HTTP 401)'
		});
		expect(state.items).toHaveLength(1);
		if (state.items[0].type !== 'error') return;
		expect(state.items[0].text).toContain('API key is invalid or missing');
	});

	it('merges text across tool calls into one assistant bubble', () => {
		const events: StreamEvent[] = [
			{ type: 'text_delta', delta: 'Let me check.' },
			{ type: 'tool_call', id: 'tc-1', tool: 'list_dir', input: { path: '.' } },
			{ type: 'tool_result', id: 'tc-1', tool: 'list_dir', output: 'src\nREADME.md' },
			{ type: 'text_delta', delta: 'Found it.' },
			{ type: 'done' }
		];
		let state = initChatState();
		for (const event of events) {
			state = reduceChatState(state, event);
		}
		expect(state.items).toHaveLength(2);
		expect(state.items[0].type).toBe('assistant');
		if (state.items[0].type !== 'assistant') return;
		expect(state.items[0].text).toBe('Let me check.Found it.');
		expect(state.items[1].type).toBe('tool');
	});

	it('clears pending tools on done', () => {
		let state = initChatState();
		state = reduceChatState(state, {
			type: 'tool_call',
			id: 'tc-1',
			tool: 'read_file',
			input: { path: 'README.md' }
		});
		state = reduceChatState(state, { type: 'done' });
		const tool = state.items[0];
		expect(tool.type).toBe('tool');
		if (tool.type !== 'tool') return;
		expect(tool.pending).toBe(false);
		expect(tool.durationMs).toBeTypeOf('number');
	});

	it('keeps one assistant across step boundaries and tool results', () => {
		const events: StreamEvent[] = [
			{ type: 'reasoning_delta', text: 'Need to inspect files.' },
			{
				type: 'step_finish',
				usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 }
			},
			{ type: 'tool_call', id: 'tc-1', tool: 'read_file', input: { path: 'main.go' } },
			{ type: 'tool_result', id: 'tc-1', tool: 'read_file', output: 'package main' },
			{ type: 'text_delta', delta: 'The file contains Go code.' }
		];
		let state = initChatState();
		for (const event of events) {
			state = reduceChatState(state, event);
		}

		expect(state.items).toHaveLength(2);
		expect(state.items[0].type).toBe('assistant');
		if (state.items[0].type !== 'assistant') return;
		expect(state.items[0].reasoning?.text).toBe('Need to inspect files.');
		expect(state.items[0].reasoning?.pending).toBe(false);
		expect(state.items[0].text).toBe('The file contains Go code.');

		expect(state.items[1].type).toBe('tool');
		if (state.items[1].type !== 'tool') return;
		expect(state.items[1].output).toBe('package main');
		expect(state.items[1].pending).toBe(false);
	});

	it('merges text that arrives after a reasoning step finish into the reasoning assistant', () => {
		const events: StreamEvent[] = [
			{ type: 'reasoning_delta', text: 'I have enough context.' },
			{
				type: 'step_finish',
				usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 }
			},
			{ type: 'text_delta', delta: 'Here is the final answer.' },
			{ type: 'done' }
		];
		let state = initChatState();
		for (const event of events) {
			state = reduceChatState(state, event);
		}

		expect(state.items).toHaveLength(1);
		const assistant = state.items[0];
		expect(assistant.type).toBe('assistant');
		if (assistant.type !== 'assistant') return;
		expect(assistant.reasoning?.text).toBe('I have enough context.');
		expect(assistant.reasoning?.pending).toBe(false);
		expect(assistant.text).toBe('Here is the final answer.');
	});

	it('accumulates subagent stream chunks and updates tool status', () => {
		let state = initChatState();
		state = reduceChatState(state, {
			type: 'subagent_started',
			child_session_id: 'child-1',
			purpose: 'Run tests',
			agent_name: 'opencode'
		});
		state = reduceChatState(state, {
			type: 'subagent_progress',
			child_session_id: 'child-1',
			progress_kind: 'message',
			progress_text: 'the '
		});
		state = reduceChatState(state, {
			type: 'subagent_progress',
			child_session_id: 'child-1',
			progress_kind: 'message',
			progress_text: 'fix.'
		});
		state = reduceChatState(state, {
			type: 'subagent_progress',
			child_session_id: 'child-1',
			progress_kind: 'tool_call',
			progress_text: 'bash (pending)'
		});
		state = reduceChatState(state, {
			type: 'subagent_progress',
			child_session_id: 'child-1',
			progress_kind: 'tool_call_update',
			progress_text: 'bash (in_progress)'
		});

		const card = state.items.find((item) => item.type === 'subagent');
		expect(card?.type).toBe('subagent');
		if (card?.type !== 'subagent') return;
		expect(card.progress).toEqual([
			{ kind: 'stream', channel: 'message', text: 'the fix.' },
			{ kind: 'tool', title: 'bash', status: 'in_progress' }
		]);
	});

	it('merges message progress after tool entries into the same stream', () => {
		let state = initChatState();
		state = reduceChatState(state, {
			type: 'subagent_started',
			child_session_id: 'child-1',
			purpose: 'Run tests',
			agent_name: 'opencode'
		});
		state = reduceChatState(state, {
			type: 'subagent_progress',
			child_session_id: 'child-1',
			progress_kind: 'message',
			progress_text: 'Which '
		});
		state = reduceChatState(state, {
			type: 'subagent_progress',
			child_session_id: 'child-1',
			progress_kind: 'tool_call',
			progress_text: 'bash (pending)'
		});
		state = reduceChatState(state, {
			type: 'subagent_progress',
			child_session_id: 'child-1',
			progress_kind: 'message',
			progress_text: 'branch?'
		});

		const card = state.items.find((item) => item.type === 'subagent');
		expect(card?.type).toBe('subagent');
		if (card?.type !== 'subagent') return;
		expect(card.progress).toEqual([
			{ kind: 'stream', channel: 'message', text: 'Which branch?' },
			{ kind: 'tool', title: 'bash', status: 'pending' }
		]);
	});
});
