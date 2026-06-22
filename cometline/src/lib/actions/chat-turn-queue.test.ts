import { describe, expect, it, vi } from 'vitest';
import { createChatTurnQueue } from './chat-turn-queue';
import type { ChatTurnPayload } from './start-chat';

describe('createChatTurnQueue', () => {
	it('runs a single turn immediately', async () => {
		const runTurn = vi.fn().mockResolvedValue(undefined);
		const queue = createChatTurnQueue(runTurn);

		await queue.enqueue('hello');

		expect(runTurn).toHaveBeenCalledTimes(1);
		expect(runTurn).toHaveBeenCalledWith({ text: 'hello' });
		expect(queue.pendingCount).toBe(0);
		expect(queue.processing).toBe(false);
	});

	it('passes images and file paths to the turn', async () => {
		const runTurn = vi.fn().mockResolvedValue(undefined);
		const queue = createChatTurnQueue(runTurn);
		const images = [{ media_type: 'image/png' as const, data: 'abc', id: '1' }];

		await queue.enqueue({ text: 'hello', images, filePaths: ['README.md'] });

		expect(runTurn).toHaveBeenCalledTimes(1);
		expect(runTurn).toHaveBeenCalledWith({ text: 'hello', images, filePaths: ['README.md'] });
	});

	it('does not place the first idle submit in the pending queue', async () => {
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (payload: ChatTurnPayload) => {
			if (payload.text === 'first') await firstGate;
		});
		const queue = createChatTurnQueue(runTurn);

		void queue.enqueue('first');

		await vi.waitFor(() => expect(queue.processing).toBe(true));
		expect(queue.pendingCount).toBe(0);
		expect(queue.pendingMessages).toEqual([]);

		releaseFirst!();
		await vi.waitFor(() => expect(queue.processing).toBe(false));
	});

	it('queues overlapping submits and runs them in order', async () => {
		const order: string[] = [];
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (payload: ChatTurnPayload) => {
			order.push(`start:${payload.text}`);
			if (payload.text === 'first') await firstGate;
			order.push(`end:${payload.text}`);
		});
		const queue = createChatTurnQueue(runTurn);

		const first = queue.enqueue('first');
		const second = queue.enqueue('second');

		await vi.waitFor(() => expect(runTurn).toHaveBeenCalledTimes(1));
		expect(queue.pendingCount).toBe(1);
		expect(queue.pendingMessages.map((item) => item.text)).toEqual(['second']);
		expect(queue.processing).toBe(true);

		releaseFirst!();
		await first;
		await second;

		expect(order).toEqual(['start:first', 'end:first', 'start:second', 'end:second']);
		expect(queue.pendingCount).toBe(0);
		expect(queue.processing).toBe(false);
	});

	it('clear drops pending turns but does not interrupt the active turn', async () => {
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (payload: ChatTurnPayload) => {
			if (payload.text === 'first') await firstGate;
		});
		const queue = createChatTurnQueue(runTurn);

		void queue.enqueue('first');
		await vi.waitFor(() => expect(queue.processing).toBe(true));

		void queue.enqueue('second');
		void queue.enqueue('third');
		expect(queue.pendingCount).toBe(2);

		queue.clear();
		expect(queue.pendingCount).toBe(0);

		releaseFirst!();
		await vi.waitFor(() => expect(queue.processing).toBe(false));
		expect(runTurn).toHaveBeenCalledTimes(1);
	});

	it('remove drops a specific queued turn', async () => {
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (payload: ChatTurnPayload) => {
			if (payload.text === 'first') await firstGate;
		});
		const queue = createChatTurnQueue(runTurn);

		void queue.enqueue('first');
		await vi.waitFor(() => expect(queue.processing).toBe(true));

		void queue.enqueue('second');
		void queue.enqueue('third');
		const toRemove = queue.pendingMessages[0].id;
		expect(queue.remove(toRemove)).toBe(true);
		expect(queue.pendingCount).toBe(1);
		expect(queue.pendingMessages[0].text).toBe('third');

		releaseFirst!();
		await vi.waitFor(() => expect(queue.processing).toBe(false));
		expect(runTurn).toHaveBeenCalledTimes(2);
	});

	it('deduplicates back-to-back identical payloads', async () => {
		const runTurn = vi.fn().mockResolvedValue(undefined);
		const queue = createChatTurnQueue(runTurn);

		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		runTurn.mockImplementationOnce(async () => {
			await firstGate;
		});

		void queue.enqueue('same');
		await vi.waitFor(() => expect(queue.processing).toBe(true));
		expect(queue.enqueue('same')).resolves.toBe(false);
		expect(queue.pendingCount).toBe(0);

		releaseFirst!();
		await vi.waitFor(() => expect(queue.processing).toBe(false));
		expect(runTurn).toHaveBeenCalledTimes(1);
	});

	it('passes displayText through queued payloads', async () => {
		const runTurn = vi.fn().mockResolvedValue(undefined);
		const queue = createChatTurnQueue(runTurn);

		await queue.enqueue({ text: 'agent prompt', displayText: '/job Fix login' });

		expect(runTurn).toHaveBeenCalledWith({
			text: 'agent prompt',
			displayText: '/job Fix login'
		});
	});
});
