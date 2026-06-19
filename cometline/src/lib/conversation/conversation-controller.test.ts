import { describe, expect, it, vi, beforeEach, type Mock } from 'vitest';
import {
	createConversationController,
	refreshConversationSession,
	type ConversationControllerDeps,
	type ConversationFlightAdapter
} from './conversation-controller';
import { chatStore } from '$lib/stores/chat.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

vi.mock('$lib/client/cometmind', () => ({
	getSession: vi.fn().mockResolvedValue({ id: 'sess-1', title: 'Updated' })
}));

describe('createConversationController', () => {
	beforeEach(() => {
		chatStore.clear();
		shellStore.centerComposer();
	});

	function createDeps(overrides?: {
		sessionId?: string;
		hasVisibleConversation?: boolean;
		send?: Mock<ConversationControllerDeps['send']>;
		refreshSession?: Mock<ConversationControllerDeps['refreshSession']>;
		flight?: {
			onUserMessageFlight: Mock<ConversationFlightAdapter['onUserMessageFlight']>;
			onFirstTurnComplete?: Mock<NonNullable<ConversationFlightAdapter['onFirstTurnComplete']>>;
		};
		onAwaitingFirstAssistantChange?: Mock<NonNullable<ConversationControllerDeps['onAwaitingFirstAssistantChange']>>;
	}) {
		const send = overrides?.send ?? vi.fn().mockResolvedValue(undefined);
		const refreshSession = overrides?.refreshSession ?? vi.fn().mockResolvedValue(undefined);
		let hasVisible = overrides?.hasVisibleConversation ?? false;
		const sessionId = overrides?.sessionId ?? 'sess-1';

		const controller = createConversationController({
			getSessionId: () => sessionId,
			getHasVisibleConversation: () => hasVisible,
			send,
			refreshSession,
			flight: overrides?.flight,
			onAwaitingFirstAssistantChange: overrides?.onAwaitingFirstAssistantChange
		});

		return {
			controller,
			send,
			refreshSession,
			setHasVisible: (value: boolean) => {
				hasVisible = value;
			}
		};
	}

	it('sends a first-turn message with skipUser when flight is enabled', async () => {
		const onUserMessageFlight = vi.fn().mockResolvedValue(undefined);
		const { controller, send, refreshSession } = createDeps({
			flight: { onUserMessageFlight }
		});

		await controller.enqueue('hello');

		expect(onUserMessageFlight).toHaveBeenCalledWith('hello', { firstTurn: true });
		expect(send).toHaveBeenCalledWith('sess-1', 'hello', { skipUser: true });
		expect(refreshSession).toHaveBeenCalledWith('sess-1');
	});

	it('skips user item on subsequent turns when flight is enabled', async () => {
		const onUserMessageFlight = vi.fn().mockResolvedValue(undefined);
		const { controller, send } = createDeps({
			hasVisibleConversation: true,
			flight: { onUserMessageFlight }
		});

		await controller.enqueue('hello again');

		expect(onUserMessageFlight).toHaveBeenCalledWith('hello again', { firstTurn: false });
		expect(send).toHaveBeenCalledWith('sess-1', 'hello again', { skipUser: true });
	});

	it('does not skip user on subsequent turns without flight', async () => {
		const { controller, send } = createDeps({ hasVisibleConversation: true });

		await controller.enqueue('hello again');

		expect(send).toHaveBeenCalledWith('sess-1', 'hello again', { skipUser: false });
	});

	it('passes file paths through to send', async () => {
		const { controller, send } = createDeps({ hasVisibleConversation: true });
		const images = [{ media_type: 'image/png' as const, data: 'abc', id: '1' }];

		await controller.enqueue('review', images, ['README.md']);

		expect(send).toHaveBeenCalledWith(
			'sess-1',
			{ text: 'review', images, filePaths: ['README.md'] },
			{ skipUser: false }
		);
	});

	it('calls onFirstTurnComplete and clears awaiting state after first turn', async () => {
		const onFirstTurnComplete = vi.fn();
		const onAwaitingFirstAssistantChange = vi.fn();
		const { controller } = createDeps({
			flight: {
				onUserMessageFlight: vi.fn().mockResolvedValue(undefined),
				onFirstTurnComplete
			},
			onAwaitingFirstAssistantChange
		});

		await controller.enqueue('first');

		expect(onFirstTurnComplete).toHaveBeenCalled();
		expect(onAwaitingFirstAssistantChange).toHaveBeenCalledWith(false);
	});

	it('queues overlapping submits and runs them in order', async () => {
		const order: string[] = [];
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const send = vi.fn().mockImplementation(async (sessionId: string, payload: string | { text: string }) => {
			const text = typeof payload === 'string' ? payload : payload.text;
			order.push(`start:${sessionId}:${text}`);
			if (text === 'first') await firstGate;
			order.push(`end:${sessionId}:${text}`);
		});
		const { controller } = createDeps({ send });

		const first = controller.enqueue('first');
		const second = controller.enqueue('second');

		await vi.waitFor(() => expect(send).toHaveBeenCalledTimes(1));
		expect(controller.pendingCount).toBe(1);

		releaseFirst!();
		await first;
		await second;

		expect(order).toEqual([
			'start:sess-1:first',
			'end:sess-1:first',
			'start:sess-1:second',
			'end:sess-1:second'
		]);
	});

	it('consumes pending first message on mount', async () => {
		sessionStore.queuePendingMessage('sess-1', 'from home', undefined);
		const send = vi.fn().mockResolvedValue(undefined);
		const { controller } = createDeps({ send });

		controller.onMount();

		await vi.waitFor(() => expect(send).toHaveBeenCalled());
		expect(send).toHaveBeenCalledWith('sess-1', 'from home', { skipUser: true });
		expect(sessionStore.hasPendingMessage('sess-1')).toBe(false);
	});

	it('keeps pending first messages isolated by session', async () => {
		sessionStore.queuePendingMessage('sess-1', 'first session', undefined);
		sessionStore.queuePendingMessage('sess-2', 'second session', undefined);
		const send1 = vi.fn().mockResolvedValue(undefined);
		const send2 = vi.fn().mockResolvedValue(undefined);

		createDeps({ sessionId: 'sess-1', send: send1 }).controller.onMount();
		createDeps({ sessionId: 'sess-2', send: send2 }).controller.onMount();

		await vi.waitFor(() => expect(send1).toHaveBeenCalled());
		await vi.waitFor(() => expect(send2).toHaveBeenCalled());
		expect(send1).toHaveBeenCalledWith('sess-1', 'first session', { skipUser: true });
		expect(send2).toHaveBeenCalledWith('sess-2', 'second session', { skipUser: true });
		expect(sessionStore.hasPendingMessage('sess-1')).toBe(false);
		expect(sessionStore.hasPendingMessage('sess-2')).toBe(false);
	});

	it('loads transcript on mount when no pending message and cache is empty', async () => {
		const loadSpy = vi.spyOn(chatStore, 'loadTranscript').mockResolvedValue(undefined);
		const { controller } = createDeps();

		controller.onMount();

		expect(loadSpy).toHaveBeenCalledWith('sess-1');
		loadSpy.mockRestore();
	});

	it('skips transcript load on mount when session has in-flight turn', async () => {
		const loadSpy = vi.spyOn(chatStore, 'loadTranscript').mockResolvedValue(undefined);
		vi.spyOn(chatStore, 'hasInFlightTurn').mockReturnValue(true);
		const { controller } = createDeps();

		controller.onMount();

		expect(loadSpy).not.toHaveBeenCalled();
		loadSpy.mockRestore();
	});

	it('sends to the session that enqueued the turn even if getSessionId changes during flight', async () => {
		let currentSessionId = 'sess-a';
		let releaseFlight: (() => void) | undefined;
		const flightGate = new Promise<void>((resolve) => {
			releaseFlight = resolve;
		});
		const send = vi.fn().mockResolvedValue(undefined);
		const onUserMessageFlight = vi.fn().mockImplementation(async () => {
			currentSessionId = 'sess-b';
			await flightGate;
		});

		const controller = createConversationController({
			getSessionId: () => currentSessionId,
			getHasVisibleConversation: () => true,
			send,
			refreshSession: vi.fn().mockResolvedValue(undefined),
			flight: { onUserMessageFlight }
		});

		const turn = controller.enqueue('question A');
		await vi.waitFor(() => expect(onUserMessageFlight).toHaveBeenCalled());
		expect(currentSessionId).toBe('sess-b');
		expect(send).not.toHaveBeenCalled();

		releaseFlight!();
		await turn;

		expect(send).toHaveBeenCalledWith('sess-a', 'question A', { skipUser: true });
	});

	it('shouldSkipTranscriptLoad when pending message exists', () => {
		sessionStore.queuePendingMessage('sess-1', 'pending', undefined);
		const { controller } = createDeps();

		expect(controller.shouldSkipTranscriptLoad()).toBe(true);
		sessionStore.takePendingMessage('sess-1');
	});

	it('bindSession docks composer when already docked or loading', () => {
		const dockSpy = vi.spyOn(shellStore, 'dockComposer');
		const { controller } = createDeps();

		shellStore.dockComposer();
		controller.bindSession();

		expect(dockSpy).toHaveBeenCalled();
		dockSpy.mockRestore();
	});

	it('does not refresh when send throws', async () => {
		const send = vi.fn().mockRejectedValue(new Error('network'));
		const refreshSession = vi.fn().mockResolvedValue(undefined);
		const { controller } = createDeps({ send, refreshSession });

		await expect(controller.enqueue('oops')).rejects.toThrow('network');
		expect(refreshSession).not.toHaveBeenCalled();
	});

	it('does not keep processing true while refreshSession is in flight', async () => {
		let releaseRefresh: (() => void) | undefined;
		const refreshGate = new Promise<void>((resolve) => {
			releaseRefresh = resolve;
		});
		const send = vi.fn().mockResolvedValue(undefined);
		const refreshSession = vi.fn().mockImplementation(() => refreshGate);
		const { controller } = createDeps({ send, refreshSession });

		const turn = controller.enqueue('hello');
		await vi.waitFor(() => expect(send).toHaveBeenCalledTimes(1));
		await vi.waitFor(() => expect(controller.processing).toBe(false));
		expect(refreshSession).toHaveBeenCalled();

		releaseRefresh!();
		await turn;
	});
});

describe('refreshConversationSession', () => {
	it('updates session store on success', async () => {
		const updateSpy = vi.spyOn(sessionStore, 'updateSession');
		await refreshConversationSession('sess-1');
		expect(updateSpy).toHaveBeenCalledWith({ id: 'sess-1', title: 'Updated' });
		updateSpy.mockRestore();
	});
});
