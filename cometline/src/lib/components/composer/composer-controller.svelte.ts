import type { AgentMode, ImageAttachment } from '$lib/types';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import type { PendingUnsentDraft } from '$lib/components/composer/composer-history';

export function createComposerInputController(deps: {
	onSend: (payload: ChatTurnPayload | string) => void;
	getValue: () => string;
	getImages: () => ImageAttachment[];
	getDisabled: () => boolean;
	getHasSelectedModel: () => boolean;
	getReasoningEffort: () => string;
	getReasoningEffortOptions: () => string[];
	getAgentMode: () => AgentMode;
	clearDraft: () => void;
	applyDraft: (draft: PendingUnsentDraft) => void;
}) {
	function canSubmit() {
		return Boolean(deps.getValue().trim() || deps.getImages().length > 0);
	}

	function sendTurn(payload: ChatTurnPayload | string) {
		if (typeof payload === 'string') {
			deps.onSend({ text: payload, agentMode: deps.getAgentMode() });
			return;
		}
		deps.onSend({ ...payload, agentMode: payload.agentMode ?? deps.getAgentMode() });
	}

	function buildSubmitPayload(filePaths: string[]): ChatTurnPayload | null {
		const trimmed = deps.getValue().trim();
		if (!canSubmit() || deps.getDisabled() || !deps.getHasSelectedModel()) return null;
		const images = deps.getImages();
		const effort = deps.getReasoningEffort();
		const supportedEffort = deps.getReasoningEffortOptions().includes(effort)
			? effort
			: undefined;
		return {
			text: trimmed,
			images: images.length > 0 ? images : undefined,
			filePaths: filePaths.length > 0 ? filePaths : undefined,
			reasoningEffort: supportedEffort || undefined,
			agentMode: deps.getAgentMode()
		};
	}

	function submitDraft(filePaths: string[]) {
		const payload = buildSubmitPayload(filePaths);
		if (!payload) return false;
		sendTurn(payload);
		deps.clearDraft();
		return true;
	}

	function restoreDraft(draft: PendingUnsentDraft) {
		if (deps.getValue().trim() || deps.getImages().length > 0) return false;
		deps.applyDraft(draft);
		return true;
	}

	return {
		canSubmit,
		sendTurn,
		buildSubmitPayload,
		submitDraft,
		restoreDraft
	};
}
