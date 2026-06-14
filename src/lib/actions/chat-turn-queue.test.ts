import { describe, expect, it, vi } from 'vitest';
import { createChatTurnQueue } from './chat-turn-queue';

describe('createChatTurnQueue', () => {
	it('runs a single turn immediately', async () => {
		const runTurn = vi.fn().mockResolvedValue(undefined);
		const queue = createChatTurnQueue(runTurn);

		await queue.enqueue('hello');

		expect(runTurn).toHaveBeenCalledTimes(1);
		expect(runTurn).toHaveBeenCalledWith('hello');
		expect(queue.pendingCount).toBe(0);
		expect(queue.processing).toBe(false);
	});

	it('does not place the first idle submit in the pending queue', async () => {
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (text: string) => {
			if (text === 'first') await firstGate;
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
		const runTurn = vi.fn().mockImplementation(async (text: string) => {
			order.push(`start:${text}`);
			if (text === 'first') await firstGate;
			order.push(`end:${text}`);
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
		const runTurn = vi.fn().mockImplementation(async (text: string) => {
			if (text === 'first') await firstGate;
		});
		const queue = createChatTurnQueue(runTurn);

		const first = queue.enqueue('first');
		void queue.enqueue('second');
		await vi.waitFor(() => expect(runTurn).toHaveBeenCalledTimes(1));

		queue.clear();
		expect(queue.pendingCount).toBe(0);

		releaseFirst!();
		await first;

		expect(runTurn).toHaveBeenCalledTimes(1);
		expect(runTurn).toHaveBeenCalledWith('first');
	});

	it('removes a queued message by id without affecting the active turn', async () => {
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (text: string) => {
			if (text === 'first') await firstGate;
		});
		const queue = createChatTurnQueue(runTurn);

		const first = queue.enqueue('first');
		void queue.enqueue('second');
		void queue.enqueue('third');
		await vi.waitFor(() => expect(runTurn).toHaveBeenCalledTimes(1));

		const toRemove = queue.pendingMessages[0].id;
		expect(queue.remove(toRemove)).toBe(true);
		expect(queue.pendingCount).toBe(1);
		expect(queue.pendingMessages[0].text).toBe('third');

		releaseFirst!();
		await first;

		expect(runTurn).toHaveBeenCalledTimes(2);
		expect(runTurn).toHaveBeenNthCalledWith(2, 'third');
	});

	it('returns false when removing an unknown queued message id', () => {
		const queue = createChatTurnQueue(vi.fn().mockResolvedValue(undefined));
		expect(queue.remove('missing')).toBe(false);
	});

	it('ignores duplicate submits while the same turn is active', async () => {
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (text: string) => {
			if (text === 'hello') await firstGate;
		});
		const queue = createChatTurnQueue(runTurn);

		const first = queue.enqueue('hello');
		await vi.waitFor(() => expect(queue.processing).toBe(true));

		await expect(queue.enqueue('hello')).resolves.toBe(false);
		expect(queue.pendingCount).toBe(0);

		releaseFirst!();
		await first;

		expect(runTurn).toHaveBeenCalledTimes(1);
		expect(runTurn).toHaveBeenCalledWith('hello');
	});

	it('ignores duplicate consecutive queued messages', async () => {
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		const runTurn = vi.fn().mockImplementation(async (text: string) => {
			if (text === 'first') await firstGate;
		});
		const queue = createChatTurnQueue(runTurn);

		void queue.enqueue('first');
		await vi.waitFor(() => expect(queue.processing).toBe(true));

		await expect(queue.enqueue('second')).resolves.toBe(true);
		await expect(queue.enqueue('second')).resolves.toBe(false);
		expect(queue.pendingCount).toBe(1);
		expect(queue.pendingMessages.map((item) => item.text)).toEqual(['second']);

		releaseFirst!();
		await vi.waitFor(() => expect(queue.processing).toBe(false));

		expect(runTurn).toHaveBeenCalledTimes(2);
	});
});
