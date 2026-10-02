import type { ChatItem } from '#lib/stores/chat.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { buildAssistantResponseContext } from '#lib/features/chat/assistant-response-context.js';
import {
	firstSelectionClientRect,
	selectionPopupPosition
} from '#lib/features/workspace/selection-popup.js';

export function createAssistantStackSelection(deps: {
	getItemId: () => string;
	getStreamingAssistantId: () => string | null;
	getSessionId: () => string;
	getThreadItems: () => readonly ChatItem[];
}) {
	let selectionPopup = $state<{ top: number; left: number; text: string } | null>(null);

	function clearSelectionPopup() {
		selectionPopup = null;
	}

	function updateSelectionPopup(root: HTMLElement) {
		if (deps.getItemId() === deps.getStreamingAssistantId()) {
			clearSelectionPopup();
			return;
		}
		const selection = window.getSelection();
		if (!selection || selection.isCollapsed || selection.rangeCount === 0) {
			clearSelectionPopup();
			return;
		}
		if (!root.contains(selection.anchorNode) || !root.contains(selection.focusNode)) {
			clearSelectionPopup();
			return;
		}
		const text = selection.toString().trim();
		if (!text) {
			clearSelectionPopup();
			return;
		}
		selectionPopup = {
			...selectionPopupPosition(
				firstSelectionClientRect(selection.getRangeAt(0)),
				window.innerWidth
			),
			text
		};
	}

	function selectableResponse(node: HTMLElement) {
		const update = () => updateSelectionPopup(node);
		node.addEventListener('mouseup', update);
		node.addEventListener('keyup', update);
		return {
			destroy() {
				node.removeEventListener('mouseup', update);
				node.removeEventListener('keyup', update);
			}
		};
	}

	function addSelectionToChat() {
		if (!selectionPopup) return;
		const contextRef = buildAssistantResponseContext({
			sessionId: deps.getSessionId(),
			itemId: deps.getItemId(),
			items: deps.getThreadItems(),
			selectedText: selectionPopup.text
		});
		if (contextRef) {
			shellStore.addWebContextForActive(contextRef);
			shellStore.requestComposerFocus();
		}
		clearSelectionPopup();
		window.getSelection()?.removeAllRanges();
	}

	return {
		get selectionPopup() {
			return selectionPopup;
		},
		clearSelectionPopup,
		selectableResponse,
		addSelectionToChat
	};
}
