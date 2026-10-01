import { matchesShortcut, type KeyboardShortcuts } from '$lib/keyboard-shortcuts';
import type { ImageAttachment } from '$lib/types';
import { nextAttachmentRemoval } from '$lib/features/composer/composer-attachment-keydown';

export type ComposerKeydownDeps = {
	getShortcuts: () => KeyboardShortcuts;
	cycleReasoningEffort: () => void;
	handleSlashMenuKeydown: (e: KeyboardEvent) => boolean;
	handleMentionMenuKeydown: (e: KeyboardEvent) => boolean;
	cycleAgentMode: () => void;
	getValue: () => string;
	getImages: () => ImageAttachment[];
	getPendingWebContextCount: () => number;
	removeImage: (id: string) => void;
	removeWebContextAt: (index: number) => void;
	isBrowsingHistory: () => boolean;
	isCaretAtStart: () => boolean;
	isCaretAtEnd: () => boolean;
	navigateHistory: (direction: 'up' | 'down') => Promise<void>;
	isStreaming: () => boolean;
	onStop: () => void;
	submit: () => Promise<void>;
};

function isPlainKey(e: KeyboardEvent) {
	return !e.isComposing && !e.metaKey && !e.ctrlKey && !e.altKey;
}

export function handleComposerKeydown(e: KeyboardEvent, deps: ComposerKeydownDeps) {
	if (!e.isComposing && matchesShortcut(e, deps.getShortcuts().cycleReasoningEffort)) {
		e.preventDefault();
		deps.cycleReasoningEffort();
		return;
	}
	if (deps.handleSlashMenuKeydown(e)) return;
	if (deps.handleMentionMenuKeydown(e)) return;
	// Plain Tab toggles the agent mode only when no slash/mention menu owns
	// the key. Shift+Tab and modified combinations keep native behavior.
	if (isPlainKey(e) && e.key === 'Tab' && !e.shiftKey) {
		e.preventDefault();
		deps.cycleAgentMode();
		return;
	}
	if (isPlainKey(e) && (e.key === 'Backspace' || e.key === 'Delete')) {
		const sel = window.getSelection();
		if (!sel || sel.isCollapsed) {
			const removal = nextAttachmentRemoval(
				deps.getValue(),
				deps.getImages(),
				deps.getPendingWebContextCount()
			);
			if (removal?.kind === 'image') {
				e.preventDefault();
				deps.removeImage(removal.id);
				return;
			}
			if (removal?.kind === 'webContext') {
				e.preventDefault();
				deps.removeWebContextAt(removal.index);
				return;
			}
		}
	}
	if (isPlainKey(e) && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
		const inHistory = deps.isBrowsingHistory();
		const canStepUp = e.key === 'ArrowUp' && deps.isCaretAtStart();
		const canStepDown = e.key === 'ArrowDown' && inHistory && deps.isCaretAtEnd();
		if (canStepUp || canStepDown) {
			e.preventDefault();
			void deps.navigateHistory(e.key === 'ArrowUp' ? 'up' : 'down');
			return;
		}
	}
	if (matchesShortcut(e, deps.getShortcuts().stopResponse) && deps.isStreaming()) {
		const sel = window.getSelection();
		if (!sel || sel.isCollapsed) {
			e.preventDefault();
			deps.onStop();
			return;
		}
	}
	if (!e.isComposing && matchesShortcut(e, deps.getShortcuts().insertNewline)) {
		return;
	}
	if (!e.isComposing && matchesShortcut(e, deps.getShortcuts().sendMessage)) {
		e.preventDefault();
		void deps.submit();
	}
}
