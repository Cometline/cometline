import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
	createConversationController,
	refreshConversationSession
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
		hasVisibleConversation?: boolean;
		send?: ReturnType<typeof vi.fn>;
		refreshSession?: ReturnType<typeof vi.fn>;
		flight?: {
			onUserMessageFlight: ReturnType<typeof vi.fn>;
			onFirstTurnComplete?: ReturnType<typeof vi.fn>;
		};
		onAwaitingFirstAssistantChange?: ReturnType<typeof vi.fn>;
	}) {
		const send = overrides?.send ?? vi.fn().mockResolvedValue(undefined);
		const refreshSession = overrides?.refreshSession ?? vi.fn().mockResolvedValue(undefined);
		let hasVisible = overrides?.hasVisibleConversation ?? false;

		const controller = createConversationController({
			getSessionId: () => 'sess-1',
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
		expect(send).toHaveBeenCalledWith('hello', { skipUser: true });
		expect(refreshSession).toHaveBeenCalled();
	});

	it('skips user item on subsequent turns when flight is enabled', async () => {
		const onUserMessageFlight = vi.fn().mockResolvedValue(undefined);
		const { controller, send } = createDeps({
			hasVisibleConversation: true,
			flight: { onUserMessageFlight }
		});

		await controller.enqueue('hello again');

		expect(onUserMessageFlight).toHaveBeenCalledWith('hello again', { firstTurn: false });
		expect(send).toHaveBeenCalledWith('hello again', { skipUser: true });
	});

	it('does not skip user on subsequent turns without flight', async () => {
		const { controller, send } = createDeps({ hasVisibleConversation: true });

		await controller.enqueue('hello again');

		expect(send).toHaveBeenCalledWith('hello again', { skipUser: false });
	});

	it('passes file paths through to send', async () => {
		const { controller, send } = createDeps({ hasVisibleConversation: true });
		const images = [{ media_type: 'image/png' as const, data: 'abc', id: '1' }];

		await controller.enqueue('review', images, ['README.md']);

		expect(send).toHaveBeenCalledWith(
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
		const send = vi.fn().mockImplementation(async (payload: string | { text: string }) => {
			const text = typeof payload === 'string' ? payload : payload.text;
			order.push(`start:${text}`);
			if (text === 'first') await firstGate;
			order.push(`end:${text}`);
		});
		const { controller } = createDeps({ send });

		const first = controller.enqueue('first');
		const second = controller.enqueue('second');

		await vi.waitFor(() => expect(send).toHaveBeenCalledTimes(1));
		expect(controller.pendingCount).toBe(1);

		releaseFirst!();
		await first;
		await second;

		expect(order).toEqual(['start:first', 'end:first', 'start:second', 'end:second']);
	});

	it('consumes pending first message on mount', async () => {
		sessionStore.queuePendingMessage('sess-1', 'from home', undefined);
		const send = vi.fn().mockResolvedValue(undefined);
		const { controller } = createDeps({ send });

		controller.onMount();

		await vi.waitFor(() => expect(send).toHaveBeenCalled());
		expect(send).toHaveBeenCalledWith('from home', { skipUser: true });
		expect(sessionStore.hasPendingMessage('sess-1')).toBe(false);
	});

	it('loads transcript on mount when no pending message', async () => {
		const loadSpy = vi.spyOn(chatStore, 'loadTranscript').mockResolvedValue(undefined);
		const { controller } = createDeps();

		controller.onMount();

		expect(loadSpy).toHaveBeenCalledWith('sess-1');
		loadSpy.mockRestore();
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

	it('keeps processing true while refreshSession is in flight after send completes', async () => {
		let releaseRefresh: (() => void) | undefined;
		const refreshGate = new Promise<void>((resolve) => {
			releaseRefresh = resolve;
		});
		const send = vi.fn().mockResolvedValue(undefined);
		const refreshSession = vi.fn().mockImplementation(async () => {
			await refreshGate;
		});
		const { controller } = createDeps({ send, refreshSession });

		const turn = controller.enqueue('hello');
		await vi.waitFor(() => expect(send).toHaveBeenCalledTimes(1));
		expect(controller.processing).toBe(true);

		releaseRefresh!();
		await turn;

		expect(controller.processing).toBe(false);
		expect(refreshSession).toHaveBeenCalled();
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
