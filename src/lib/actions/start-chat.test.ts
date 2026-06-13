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

	it('does not skip the user item on subsequent turns', async () => {
		const adapter = createAdapter({ hasVisibleConversation: true });
		await startChat(adapter, 'hello again');

		expect(adapter.send).toHaveBeenCalledWith('hello again', { skipUser: false });
		expect(adapter.refreshSession).toHaveBeenCalled();
	});

	it('runs the first-turn flight pre-step only on the first turn', async () => {
		const onFirstTurnStart = vi.fn().mockResolvedValue(undefined);
		const onFirstTurnComplete = vi.fn();
		const adapter = createAdapter({ onFirstTurnStart, onFirstTurnComplete });

		await startChat(adapter, 'first');

		expect(onFirstTurnStart).toHaveBeenCalledWith('first');
		expect(onFirstTurnComplete).toHaveBeenCalled();
	});

	it('skips first-turn hooks on subsequent turns', async () => {
		const onFirstTurnStart = vi.fn().mockResolvedValue(undefined);
		const onFirstTurnComplete = vi.fn();
		const adapter = createAdapter({
			hasVisibleConversation: true,
			onFirstTurnStart,
			onFirstTurnComplete
		});

		await startChat(adapter, 'second');

		expect(onFirstTurnStart).not.toHaveBeenCalled();
		expect(onFirstTurnComplete).not.toHaveBeenCalled();
	});

	it('sends before calling onFirstTurnComplete', async () => {
		const order: string[] = [];
		const adapter = createAdapter({
			onFirstTurnStart: vi.fn().mockImplementation(async () => {
				order.push('start');
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

		expect(order).toEqual(['start', 'send', 'complete', 'refresh']);
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
			onFirstTurnStart: vi.fn().mockRejectedValue(new Error('flight failed'))
		});

		await expect(startChat(adapter, 'oops')).rejects.toThrow('flight failed');
		expect(adapter.send).not.toHaveBeenCalled();
		expect(adapter.refreshSession).not.toHaveBeenCalled();
	});
});
