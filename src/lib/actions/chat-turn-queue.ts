/**
 * Serializes chat turns so only one startChat/send runs at a time.
 * Additional submits while busy are queued FIFO and drained automatically.
 */

export interface QueuedMessage {
	id: string;
	text: string;
}

export interface ChatTurnQueue {
	enqueue(text: string): Promise<boolean>;
	remove(id: string): boolean;
	clear(): void;
	readonly pendingCount: number;
	readonly pendingMessages: readonly QueuedMessage[];
	readonly processing: boolean;
}

export function createChatTurnQueue(
	runTurn: (text: string) => Promise<void>,
	onChange?: () => void
): ChatTurnQueue {
	let queue: QueuedMessage[] = [];
	let processing = false;
	let activeTurnText: string | null = null;
	let nextID = 0;

	function notifyChange() {
		onChange?.();
	}

	function createQueuedMessage(text: string): QueuedMessage {
		nextID += 1;
		return { id: `queued-${Date.now()}-${nextID}`, text };
	}

	function isDuplicate(text: string): boolean {
		if (activeTurnText === text) return true;
		return queue.at(-1)?.text === text;
	}

	function queueTurn(text: string): boolean {
		if (isDuplicate(text)) return false;
		queue.push(createQueuedMessage(text));
		notifyChange();
		return true;
	}

	async function runTurnTracked(text: string): Promise<void> {
		activeTurnText = text;
		try {
			await runTurn(text);
		} finally {
			if (activeTurnText === text) activeTurnText = null;
		}
	}

	async function runLoop(initialText?: string): Promise<boolean> {
		if (processing) {
			if (initialText !== undefined) return queueTurn(initialText);
			return false;
		}

		processing = true;
		notifyChange();
		try {
			if (initialText !== undefined) {
				await runTurnTracked(initialText);
			}
			while (queue.length > 0) {
				const { text } = queue.shift()!;
				notifyChange();
				await runTurnTracked(text);
			}
		} finally {
			activeTurnText = null;
			processing = false;
			notifyChange();
		}
		return true;
	}

	return {
		get pendingCount() {
			return queue.length;
		},
		get pendingMessages() {
			return queue;
		},
		get processing() {
			return processing;
		},
		enqueue(text: string) {
			if (processing) return Promise.resolve(queueTurn(text));
			return runLoop(text);
		},
		remove(id: string) {
			const index = queue.findIndex((item) => item.id === id);
			if (index < 0) return false;
			queue.splice(index, 1);
			notifyChange();
			return true;
		},
		clear() {
			queue = [];
			notifyChange();
		}
	};
}
