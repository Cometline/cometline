import type { ImageAttachment } from '$lib/types';
import type { ChatTurnPayload } from '$lib/actions/start-chat';

export function createComposerInputController(deps: {
	onSend: (payload: ChatTurnPayload | string) => void;
	getValue: () => string;
	getImages: () => ImageAttachment[];
	getDisabled: () => boolean;
	getHasSelectedModel: () => boolean;
	clearDraft: () => void;
}) {
	function canSubmit() {
		return Boolean(deps.getValue().trim() || deps.getImages().length > 0);
	}

	function sendTurn(payload: ChatTurnPayload | string) {
		if (typeof payload === 'string') {
			deps.onSend({ text: payload });
			return;
		}
		deps.onSend(payload);
	}

	function buildSubmitPayload(filePaths: string[]): ChatTurnPayload | null {
		const trimmed = deps.getValue().trim();
		if (!canSubmit() || deps.getDisabled() || !deps.getHasSelectedModel()) return null;
		const images = deps.getImages();
		return {
			text: trimmed,
			images: images.length > 0 ? images : undefined,
			filePaths: filePaths.length > 0 ? filePaths : undefined
		};
	}

	function submitDraft(filePaths: string[]) {
		const payload = buildSubmitPayload(filePaths);
		if (!payload) return false;
		sendTurn(payload);
		deps.clearDraft();
		return true;
	}

	return {
		canSubmit,
		sendTurn,
		buildSubmitPayload,
		submitDraft
	};
}
