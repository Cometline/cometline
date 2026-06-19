import { describe, expect, it, vi } from 'vitest';
import { startChat, type StartChatAdapter } from './start-chat';

function createAdapter(overrides?: Partial<StartChatAdapter>): StartChatAdapter {
	return {
		sessionId: 'sess-1',
		hasVisibleConversation: false,
		send: vi.fn().mockResolvedValue(undefined),
		refreshSession: vi.fn().mockResolvedValue(undefined),
		...overrides
	};
}

describe('startChat', () => {
	it('sends a first-turn message with skipUser and refreshes', async () => {
		const adapter = createAdapter();
		await startChat(adapter, 'hello');

		expect(adapter.send).toHaveBeenCalledWith('hello', { skipUser: true });
		expect(adapter.refreshSession).toHaveBeenCalled();
	});

	it('skips the user item on subsequent turns when flight is enabled', async () => {
		const onUserMessageFlight = vi.fn().mockResolvedValue(undefined);
		const adapter = createAdapter({
			hasVisibleConversation: true,
			onUserMessageFlight
		});
		await startChat(adapter, 'hello again');

		expect(onUserMessageFlight).toHaveBeenCalledWith('hello again', { firstTurn: false });
		expect(adapter.send).toHaveBeenCalledWith('hello again', { skipUser: true });
		expect(adapter.refreshSession).toHaveBeenCalled();
	});

	it('does not skip the user item on subsequent turns without flight', async () => {
		const adapter = createAdapter({ hasVisibleConversation: true });
		await startChat(adapter, 'hello again');

		expect(adapter.send).toHaveBeenCalledWith('hello again', { skipUser: false });
		expect(adapter.refreshSession).toHaveBeenCalled();
	});

	it('passes images and file paths to send', async () => {
		const adapter = createAdapter({ hasVisibleConversation: true });
		const images = [{ media_type: 'image/png' as const, data: 'abc', id: '1' }];

		await startChat(adapter, { text: 'review', images, filePaths: ['README.md'] });

		expect(adapter.send).toHaveBeenCalledWith(
			{ text: 'review', images, filePaths: ['README.md'] },
			{ skipUser: false }
		);
	});

	it('runs the user message flight on every turn when provided', async () => {
		const onUserMessageFlight = vi.fn().mockResolvedValue(undefined);
		const onFirstTurnComplete = vi.fn();
		const adapter = createAdapter({ onUserMessageFlight, onFirstTurnComplete });

		await startChat(adapter, 'first');

		expect(onUserMessageFlight).toHaveBeenCalledWith('first', { firstTurn: true });
		expect(onFirstTurnComplete).toHaveBeenCalled();
	});

	it('skips first-turn complete hook on subsequent turns', async () => {
		const onUserMessageFlight = vi.fn().mockResolvedValue(undefined);
		const onFirstTurnComplete = vi.fn();
		const adapter = createAdapter({
			hasVisibleConversation: true,
			onUserMessageFlight,
			onFirstTurnComplete
		});

		await startChat(adapter, 'second');

		expect(onUserMessageFlight).toHaveBeenCalledWith('second', { firstTurn: false });
		expect(onFirstTurnComplete).not.toHaveBeenCalled();
	});

	it('sends before calling onFirstTurnComplete', async () => {
		const order: string[] = [];
		const adapter = createAdapter({
			onUserMessageFlight: vi.fn().mockImplementation(async () => {
				order.push('flight');
			}),
			send: vi.fn().mockImplementation(async () => {
				order.push('send');
			}),
			onFirstTurnComplete: vi.fn().mockImplementation(() => {
				order.push('complete');
			}),
			refreshSession: vi.fn().mockImplementation(async () => {
				order.push('refresh');
			})
		});

		await startChat(adapter, 'ordered');

		expect(order).toEqual(['flight', 'send', 'complete', 'refresh']);
	});

	it('does not refresh when send throws', async () => {
		const adapter = createAdapter({
			send: vi.fn().mockRejectedValue(new Error('network'))
		});

		await expect(startChat(adapter, 'oops')).rejects.toThrow('network');
		expect(adapter.refreshSession).not.toHaveBeenCalled();
	});

	it('does not send or refresh when flight pre-step throws', async () => {
		const adapter = createAdapter({
			onUserMessageFlight: vi.fn().mockRejectedValue(new Error('flight failed'))
		});

		await expect(startChat(adapter, 'oops')).rejects.toThrow('flight failed');
		expect(adapter.send).not.toHaveBeenCalled();
		expect(adapter.refreshSession).not.toHaveBeenCalled();
	});
});
