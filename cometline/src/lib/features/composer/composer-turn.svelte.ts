import type { ChatTurnPayload, WebContext } from '$lib/actions/start-chat';
import { handleComposerKeydown } from '$lib/features/composer/composer-keydown';
import type { ComposerInputRef } from '$lib/features/composer/composer-input-ref';
import { nextReasoningEffort } from '$lib/features/composer/reasoning-effort';
import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { getReasoningEffort, setReasoningEffort } from '$lib/stores/reasoning-effort.svelte';
import type { ImageAttachment } from '$lib/types';

type SlashResolver = {
	resolveSubmitAction: (trimmed: string) => {
		kind: 'handled' | 'message';
		text?: string;
		displayText?: string;
	};
	handleMenuKeydown: (e: KeyboardEvent) => boolean;
};

export function createComposerTurnController(deps: {
	getValue: () => string;
	getImages: () => ImageAttachment[];
	getInput: () => ComposerInputRef | null;
	getSessionId: () => string;
	getDisabled: () => boolean;
	getStreaming: () => boolean;
	getCanSubmit: () => boolean;
	getSlash: () => SlashResolver;
	getMentionKeydown: () => (e: KeyboardEvent) => boolean;
	cycleAgentMode: () => void;
	isBrowsingHistory: () => boolean;
	navigateHistory: (direction: 'up' | 'down') => Promise<void>;
	sendTurn: (payload: ChatTurnPayload | string) => void;
	clearDraft: () => void;
	skipEmptyStash: (clear: () => void) => void;
	removeImage: (id: string) => void;
	onStop?: () => void;
	onModelChange?: (option: ModelOption) => void | Promise<void>;
}) {
	let resolvingWebContext = $state(false);
	const pendingWebContexts = $derived(shellStore.pendingWebContexts);

	function currentReasoningEffort() {
		const current = getReasoningEffort(deps.getSessionId());
		return (modelStore.selected?.reasoningEffortOptions ?? []).includes(current) ? current : '';
	}

	function cycleReasoningEffort() {
		const supported = modelStore.selected?.reasoningEffortOptions ?? [];
		if (supported.length === 0) return;
		const next = nextReasoningEffort(currentReasoningEffort(), supported);
		setReasoningEffort(deps.getSessionId(), next);
	}

	async function changeModel(option: ModelOption) {
		const current = getReasoningEffort(deps.getSessionId());
		if (current && !(option.reasoningEffortOptions ?? []).includes(current)) {
			setReasoningEffort(deps.getSessionId(), '');
		}
		await deps.onModelChange?.(option);
	}

	async function submit() {
		const trimmed = deps.getValue().trim();
		const action = deps.getSlash().resolveSubmitAction(trimmed);
		if (action.kind === 'handled') return;
		if (
			!deps.getCanSubmit() ||
			deps.getDisabled() ||
			resolvingWebContext ||
			!modelStore.selected
		) {
			return;
		}
		const filePaths = deps.getInput()?.getFilePaths() ?? [];
		const contextsBeforeResolve = pendingWebContexts.length;
		resolvingWebContext = contextsBeforeResolve > 0;
		let webContexts: WebContext[] = [];
		try {
			webContexts = contextsBeforeResolve
				? await shellStore.resolvePendingWebContextsForActive()
				: [];
		} finally {
			resolvingWebContext = false;
		}
		const displayText =
			action.displayText ?? (webContexts.length > 0 ? action.text : undefined);
		const images = deps.getImages();
		deps.sendTurn({
			text: action.text ?? trimmed,
			displayText,
			images: images.length > 0 ? images : undefined,
			filePaths: filePaths.length > 0 ? filePaths : undefined,
			webContexts: webContexts.length > 0 ? webContexts : undefined
		});
		if (contextsBeforeResolve > 0) shellStore.clearWebContextForActive();
		deps.skipEmptyStash(() => {
			deps.getInput()?.clear();
			deps.clearDraft();
		});
	}

	function onKeydown(e: KeyboardEvent) {
		handleComposerKeydown(e, {
			getShortcuts: () => settingsStore.settings.shortcuts,
			cycleReasoningEffort,
			handleSlashMenuKeydown: (event) => deps.getSlash().handleMenuKeydown(event),
			handleMentionMenuKeydown: deps.getMentionKeydown(),
			cycleAgentMode: deps.cycleAgentMode,
			getValue: deps.getValue,
			getImages: deps.getImages,
			getPendingWebContextCount: () => pendingWebContexts.length,
			removeImage: deps.removeImage,
			removeWebContextAt: (index) => shellStore.removeWebContextAt(index),
			isBrowsingHistory: deps.isBrowsingHistory,
			isCaretAtStart: () => deps.getInput()?.isCaretAtStart() ?? true,
			isCaretAtEnd: () => deps.getInput()?.isCaretAtEnd() ?? true,
			navigateHistory: deps.navigateHistory,
			isStreaming: deps.getStreaming,
			onStop: () => deps.onStop?.(),
			submit
		});
	}

	return {
		get resolvingWebContext() {
			return resolvingWebContext;
		},
		currentReasoningEffort,
		cycleReasoningEffort,
		changeModel,
		submit,
		onKeydown
	};
}
