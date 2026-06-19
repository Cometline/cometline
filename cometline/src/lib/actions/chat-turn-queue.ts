/**
 * Serializes chat turns so only one startChat/send runs at a time.
 * Additional submits while busy are queued FIFO and drained automatically.
 */

import type { ImageAttachment } from '$lib/types';

export interface QueuedMessage {
	id: string;
	text: string;
	images?: ImageAttachment[];
	filePaths?: string[];
}

export interface ChatTurnQueue {
	enqueue(text: string, images?: ImageAttachment[], filePaths?: string[]): Promise<boolean>;
	remove(id: string): boolean;
	clear(): void;
	readonly pendingCount: number;
	readonly pendingMessages: readonly QueuedMessage[];
	readonly processing: boolean;
}

export function createChatTurnQueue(
	runTurn: (text: string, images?: ImageAttachment[], filePaths?: string[]) => Promise<void>,
	onChange?: () => void
): ChatTurnQueue {
	let queue: QueuedMessage[] = [];
	let processing = false;
	let activeTurnText: string | null = null;
	let nextID = 0;

	function notifyChange() {
		onChange?.();
	}

	function createQueuedMessage(
		text: string,
		images?: ImageAttachment[],
		filePaths?: string[]
	): QueuedMessage {
		nextID += 1;
		return { id: `queued-${Date.now()}-${nextID}`, text, images, filePaths };
	}

	function signature(text: string, images?: ImageAttachment[], filePaths?: string[]): string {
		return JSON.stringify({
			text,
			images: images?.map((image) => image.data) ?? [],
			filePaths: filePaths ?? []
		});
	}

	function isDuplicate(text: string, images?: ImageAttachment[], filePaths?: string[]): boolean {
		const sig = signature(text, images, filePaths);
		if (activeTurnText === sig) return true;
		const last = queue.at(-1);
		return last ? signature(last.text, last.images, last.filePaths) === sig : false;
	}

	function queueTurn(text: string, images?: ImageAttachment[], filePaths?: string[]): boolean {
		if (isDuplicate(text, images, filePaths)) return false;
		queue.push(createQueuedMessage(text, images, filePaths));
		notifyChange();
		return true;
	}

	async function runTurnTracked(
		text: string,
		images?: ImageAttachment[],
		filePaths?: string[]
	): Promise<void> {
		activeTurnText = signature(text, images, filePaths);
		try {
			if (images === undefined && filePaths === undefined) {
				await runTurn(text);
			} else if (filePaths === undefined) {
				await runTurn(text, images);
			} else {
				await runTurn(text, images, filePaths);
			}
		} finally {
			if (activeTurnText === signature(text, images, filePaths)) activeTurnText = null;
		}
	}

	async function runLoop(
		initialText?: string,
		initialImages?: ImageAttachment[],
		initialFilePaths?: string[]
	): Promise<boolean> {
		if (processing) {
			if (initialText !== undefined)
				return queueTurn(initialText, initialImages, initialFilePaths);
			return false;
		}

		processing = true;
		notifyChange();
		try {
			if (initialText !== undefined) {
				await runTurnTracked(initialText, initialImages, initialFilePaths);
			}
			while (queue.length > 0) {
				const { text, images, filePaths } = queue.shift()!;
				notifyChange();
				await runTurnTracked(text, images, filePaths);
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
		enqueue(text: string, images?: ImageAttachment[], filePaths?: string[]) {
			if (processing) return Promise.resolve(queueTurn(text, images, filePaths));
			return runLoop(text, images, filePaths);
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
