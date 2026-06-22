/**
 * Serializes chat turns so only one startChat/send runs at a time.
 * Additional submits while busy are queued FIFO and drained automatically.
 */

import type { ChatTurnPayload } from '$lib/actions/start-chat';

export interface QueuedMessage extends ChatTurnPayload {
	id: string;
}

export interface ChatTurnQueue {
	enqueue(payload: ChatTurnPayload | string): Promise<boolean>;
	remove(id: string): boolean;
	clear(): void;
	readonly pendingCount: number;
	readonly pendingMessages: readonly QueuedMessage[];
	readonly processing: boolean;
}

function normalizePayload(payload: ChatTurnPayload | string): ChatTurnPayload {
	return typeof payload === 'string' ? { text: payload } : payload;
}

export function createChatTurnQueue(
	runTurn: (payload: ChatTurnPayload) => Promise<void>,
	onChange?: () => void
): ChatTurnQueue {
	let queue: QueuedMessage[] = [];
	let processing = false;
	let activeTurnSignature: string | null = null;
	let nextID = 0;

	function notifyChange() {
		onChange?.();
	}

	function createQueuedMessage(payload: ChatTurnPayload): QueuedMessage {
		nextID += 1;
		return { id: `queued-${Date.now()}-${nextID}`, ...payload };
	}

	function signature(payload: ChatTurnPayload): string {
		return JSON.stringify({
			text: payload.text,
			displayText: payload.displayText ?? '',
			images: payload.images?.map((image) => image.data) ?? [],
			filePaths: payload.filePaths ?? []
		});
	}

	function isDuplicate(payload: ChatTurnPayload): boolean {
		const sig = signature(payload);
		if (activeTurnSignature === sig) return true;
		const last = queue.at(-1);
		return last ? signature(last) === sig : false;
	}

	function queueTurn(payload: ChatTurnPayload): boolean {
		if (isDuplicate(payload)) return false;
		queue.push(createQueuedMessage(payload));
		notifyChange();
		return true;
	}

	async function runTurnTracked(payload: ChatTurnPayload): Promise<void> {
		activeTurnSignature = signature(payload);
		try {
			await runTurn(payload);
		} finally {
			if (activeTurnSignature === signature(payload)) activeTurnSignature = null;
		}
	}

	async function runLoop(initialPayload?: ChatTurnPayload): Promise<boolean> {
		if (processing) {
			if (initialPayload !== undefined) return queueTurn(initialPayload);
			return false;
		}

		processing = true;
		notifyChange();
		try {
			if (initialPayload !== undefined) {
				await runTurnTracked(initialPayload);
			}
			while (queue.length > 0) {
				const { text, displayText, images, filePaths } = queue.shift()!;
				notifyChange();
				await runTurnTracked({ text, displayText, images, filePaths });
			}
		} finally {
			activeTurnSignature = null;
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
		enqueue(payload: ChatTurnPayload | string) {
			const normalized = normalizePayload(payload);
			if (processing) return Promise.resolve(queueTurn(normalized));
			return runLoop(normalized);
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
