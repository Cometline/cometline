import { tick } from 'svelte';
import type { ChatTurnPayload } from '#lib/actions/start-chat.js';
import { chatStore } from '#lib/stores/chat.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { composerHistoryStore } from '#lib/stores/composer-history.svelte.js';
import type { ImageAttachment } from '#lib/types.js';
import type { ComposerInputRef } from '#lib/features/composer/composer-input-ref.js';
import { stepHistoryIndex } from '#lib/features/composer/composer-history.js';

/** Up/Down history recall plus unsent-draft stashing for the composer. */
export function createComposerDraftHistoryController(deps: {
	getValue: () => string;
	setValue: (value: string) => void;
	getImages: () => ImageAttachment[];
	setImages: (images: ImageAttachment[]) => void;
	getInput: () => ComposerInputRef | null;
	getSessionId: () => string;
	onSessionChanged: () => void;
}) {
	let historyIndex = $state<number | null>(null);
	let historyLiveDraft = $state('');
	let historyRecallList = $state.raw<string[]>([]);
	let historyAppliedText = $state<string | null>(null);
	let trackedSessionId = $state<string | null>(null);
	let skippingEmptyStash = $state(false);
	let lastNonEmptyDraft = $state('');

	function resetHistoryBrowse() {
		historyIndex = null;
		historyLiveDraft = '';
		historyRecallList = [];
		historyAppliedText = null;
	}

	function skipEmptyStash(apply: () => void) {
		skippingEmptyStash = true;
		apply();
		void tick().then(() => {
			skippingEmptyStash = false;
		});
	}

	function applyComposerText(text: string, nextImages: ImageAttachment[] = []) {
		skipEmptyStash(() => {
			deps.setValue(text);
			deps.setImages(nextImages);
			if (text) {
				deps.getInput()?.setText(text);
			} else {
				deps.getInput()?.clear();
			}
		});
	}

	function recordSentHistory(payload: ChatTurnPayload | string) {
		const display =
			typeof payload === 'string'
				? payload.trim()
				: (payload.displayText ?? payload.text).trim();
		if (!display) return;
		const sessionId = deps.getSessionId();
		void composerHistoryStore.append({
			display,
			workspacePath: shellStore.workspacePath,
			sessionId
		});
		composerHistoryStore.clearPending(sessionId);
		resetHistoryBrowse();
		lastNonEmptyDraft = '';
	}

	function markDraftRestored(text: string) {
		resetHistoryBrowse();
		lastNonEmptyDraft = text;
	}

	function stashUnsentNow() {
		if (deps.getValue().trim() || deps.getImages().length > 0) {
			composerHistoryStore.stashUnsent(deps.getSessionId(), {
				text: deps.getValue(),
				images: deps.getImages()
			});
		}
	}

	function trackSessionChange() {
		const nextSessionId = deps.getSessionId();
		if (trackedSessionId === null) {
			trackedSessionId = nextSessionId;
			return;
		}
		if (trackedSessionId === nextSessionId) return;

		const prev = trackedSessionId;
		if (deps.getValue().trim() || deps.getImages().length > 0) {
			composerHistoryStore.stashUnsent(prev, {
				text: deps.getValue(),
				images: deps.getImages()
			});
		}
		trackedSessionId = nextSessionId;
		deps.onSessionChanged();
		resetHistoryBrowse();
		lastNonEmptyDraft = '';
		applyComposerText('');
	}

	function syncDraftStash() {
		if (
			historyIndex !== null &&
			historyAppliedText !== null &&
			deps.getValue() !== historyAppliedText
		) {
			// User edited while browsing — leave history mode.
			resetHistoryBrowse();
		}
		// While browsing history, do not treat recalled text as a live draft to stash.
		if (historyIndex !== null) return;

		const value = deps.getValue();
		const trimmed = value.trim();
		if (trimmed) {
			lastNonEmptyDraft = value;
			return;
		}
		if (skippingEmptyStash) return;
		if (!lastNonEmptyDraft.trim()) return;
		const images = deps.getImages();
		composerHistoryStore.stashUnsent(deps.getSessionId(), {
			text: lastNonEmptyDraft,
			images: images.length > 0 ? images : undefined
		});
		lastNonEmptyDraft = '';
	}

	async function navigateHistory(direction: 'up' | 'down') {
		const sessionId = deps.getSessionId();
		let list = historyRecallList;
		if (historyIndex === null) {
			const transcriptTexts =
				sessionId && chatStore.sessionID === sessionId
					? composerHistoryStore.listUserMessageTexts(chatStore.items)
					: [];
			list = await composerHistoryStore.recallTexts({
				sessionId,
				workspacePath: shellStore.workspacePath,
				transcriptUserTexts: transcriptTexts
			});
			if (list.length === 0) return;
			historyRecallList = list;
			historyLiveDraft = deps.getValue();
		}

		const next = stepHistoryIndex(historyIndex, direction, list.length);
		if (next.index === null) {
			applyComposerText(historyLiveDraft);
			resetHistoryBrowse();
			return;
		}

		historyIndex = next.index;
		const text = list[next.index] ?? '';
		historyAppliedText = text;
		const pending = composerHistoryStore.getPending(deps.getSessionId());
		const recallImages =
			pending?.text.trim() === text.trim() && pending.images?.length ? pending.images : [];
		applyComposerText(text, recallImages);
	}

	return {
		get browsing() {
			return historyIndex !== null;
		},
		skipEmptyStash,
		applyComposerText,
		recordSentHistory,
		markDraftRestored,
		stashUnsentNow,
		trackSessionChange,
		syncDraftStash,
		navigateHistory
	};
}
