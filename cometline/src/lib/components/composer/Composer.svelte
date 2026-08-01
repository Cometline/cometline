<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import { fade } from 'svelte/transition';
	import { FileText } from '@lucide/svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import type { ChatTurnPayload, WebContext } from '$lib/actions/start-chat';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';
	import RichComposerInput from '$lib/components/RichComposerInput.svelte';
	import ImageAttachments from '$lib/components/composer/ImageAttachments.svelte';
	import MessageQueuePanel from '$lib/components/composer/MessageQueuePanel.svelte';
	import ComposerSlashMenus from '$lib/components/composer/ComposerSlashMenus.svelte';
	import ComposerMentionMenu from '$lib/components/composer/ComposerMentionMenu.svelte';
	import ComposerToolbar from '$lib/components/composer/ComposerToolbar.svelte';
	import MessageContextChips from '$lib/components/chat/MessageContextChips.svelte';
	import { messageContextRefsFromPending } from '$lib/chat/message-context';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { composerHistoryStore } from '$lib/stores/composer-history.svelte';
	import { DEFAULT_CONTEXT_WINDOW_LIMIT, resolveContextWindowUsage } from '$lib/context-window';
	import { workspaceLabel } from '$lib/sessions/group-by-workspace';
	import type { ImageAttachment } from '$lib/types';
	import type { ComposerInputRef } from '$lib/components/composer/composer-input-ref';
	import { createComposerInputController } from '$lib/components/composer/composer-controller.svelte';
	import { createComposerAttachmentsController } from '$lib/components/composer/composer-attachments.svelte';
	import { createComposerMentionsController } from '$lib/components/composer/composer-mentions.svelte';
	import { createComposerSlashController } from '$lib/components/composer/composer-slash.svelte';
	import { stepHistoryIndex } from '$lib/components/composer/composer-history';
	import { nextAttachmentRemoval } from '$lib/components/composer/composer-attachment-keydown';
	import { nextReasoningEffort } from '$lib/composer/reasoning-effort';
	import { getReasoningEffort, setReasoningEffort } from '$lib/stores/reasoning-effort.svelte';

	let {
		onSend,
		onStop,
		onRemoveQueued,
		onModelChange,
		onWorkspaceChanged,
		onTranscriptCleared,
		sessionId = '',
		disabled = false,
		streaming = false,
		queuedCount = 0,
		queuedMessages = [],
		variant = 'dock',
		autofocus = true
	}: {
		onSend: (payload: ChatTurnPayload | string) => void;
		onStop?: () => void;
		onRemoveQueued?: (id: string) => void;
		onModelChange?: (option: ModelOption) => void | Promise<void>;
		onWorkspaceChanged?: () => void | Promise<void>;
		onTranscriptCleared?: () => void;
		sessionId?: string;
		disabled?: boolean;
		streaming?: boolean;
		queuedCount?: number;
		queuedMessages?: QueuedMessage[];
		variant?: 'hero' | 'dock';
		autofocus?: boolean;
	} = $props();

	let value = $state('');
	let images = $state<ImageAttachment[]>([]);
	let input = $state<RichComposerInput | null>(null);
	let skillMenu = $state<HTMLDivElement | null>(null);
	let mentionMenu = $state<HTMLDivElement | null>(null);
	let resolvingWebContext = $state(false);
	let historyIndex = $state<number | null>(null);
	let historyLiveDraft = $state('');
	let historyRecallList = $state.raw<string[]>([]);
	let historyAppliedText = $state<string | null>(null);
	let trackedSessionId = $state<string | null>(null);
	let skippingEmptyStash = $state(false);
	let lastNonEmptyDraft = $state('');
	const heroPlaceholders = [
		'Type something. Anything.',
		'Ask a question.',
		'Share a thought.',
		'Drop in a task.',
		'Bring an idea to life.'
	];
	let heroPlaceholderIndex = $state(0);

	onMount(() => {
		void composerHistoryStore.ensureLoaded();
		const rotation = window.setInterval(() => {
			heroPlaceholderIndex = (heroPlaceholderIndex + 1) % heroPlaceholders.length;
		}, 10000);

		return () => window.clearInterval(rotation);
	});

	function clearDraft() {
		value = '';
		images = [];
	}

	function resetHistoryBrowse() {
		historyIndex = null;
		historyLiveDraft = '';
		historyRecallList = [];
		historyAppliedText = null;
	}

	function applyComposerText(text: string, nextImages: ImageAttachment[] = []) {
		skippingEmptyStash = true;
		value = text;
		images = nextImages;
		if (text) {
			input?.setText(text);
		} else {
			input?.clear();
		}
		void tick().then(() => {
			skippingEmptyStash = false;
		});
	}

	const getInput = (): ComposerInputRef | null => input;

	function recordSentHistory(payload: ChatTurnPayload | string) {
		const display =
			typeof payload === 'string'
				? payload.trim()
				: (payload.displayText ?? payload.text).trim();
		if (!display) return;
		void composerHistoryStore.append({
			display,
			workspacePath: shellStore.workspacePath,
			sessionId
		});
		composerHistoryStore.clearPending(sessionId);
		resetHistoryBrowse();
		lastNonEmptyDraft = '';
	}

	const inputController = createComposerInputController({
		onSend: (payload) => {
			recordSentHistory(payload);
			onSend(payload);
		},
		getValue: () => value,
		getImages: () => images,
		getDisabled: () => disabled,
		getHasSelectedModel: () => Boolean(modelStore.selected),
		getReasoningEffort: () => getReasoningEffort(sessionId),
		clearDraft
	});

	const attachments = createComposerAttachmentsController({
		getValue: () => value,
		getImages: () => images,
		setImages: (next) => {
			images = next;
		},
		getInput
	});

	const mentions = createComposerMentionsController({
		getInput,
		getMentionMenuRef: () => mentionMenu
	});

	async function focusInput(options?: { position?: 'start' | 'end' }) {
		await tick();
		setTimeout(() => {
			const position = options?.position ?? (value.trim() ? 'end' : 'start');
			void input?.focusAsync({ position });
		}, 0);
	}

	const slash = createComposerSlashController({
		getValue: () => value,
		setValue: (next) => {
			value = next;
		},
		getInput,
		getSessionId: () => sessionId,
		getStreaming: () => streaming,
		getImages: () => images,
		setImages: (next) => {
			images = next;
		},
		sendTurn: (payload) => inputController.sendTurn(payload),
		onModelChange: (option) => onModelChange?.(option),
		onWorkspaceChanged: () => onWorkspaceChanged?.(),
		onTranscriptCleared: () => onTranscriptCleared?.(),
		setDropMessage: (message) => attachments.setDropMessage(message),
		focusInput,
		getSkillMenuRef: () => skillMenu
	});

	const canSubmit = $derived(inputController.canSubmit());
	const contextWindowUsage = $derived.by(() => {
		const items = sessionId && chatStore.sessionID === sessionId ? chatStore.items : [];
		const budget = sessionId && chatStore.sessionID === sessionId ? chatStore.contextBudget : null;
		const selected = modelStore.selected;
		return resolveContextWindowUsage({
			budget,
			items,
			draftText: value,
			contextWindowLimit: selected?.context ?? DEFAULT_CONTEXT_WINDOW_LIMIT,
			maxTokens: settingsStore.settings.cometmind.maxTokens,
			modelOutput: selected?.output ?? null
		});
	});
	const currentWorkspaceLabel = $derived(
		mentions.hasWorkspace ? workspaceLabel(shellStore.workspacePath) : ''
	);
	const pendingWebContexts = $derived(shellStore.pendingWebContexts);
	const pendingContextRefs = $derived(messageContextRefsFromPending(pendingWebContexts));

	export function focus() {
		void focusInput();
	}

	$effect(() => {
		const nextSessionId = sessionId;
		if (trackedSessionId === null) {
			trackedSessionId = nextSessionId;
			return;
		}
		if (trackedSessionId === nextSessionId) return;

		const prev = trackedSessionId;
		if (value.trim() || images.length > 0) {
			composerHistoryStore.stashUnsent(prev, { text: value, images });
		}
		trackedSessionId = nextSessionId;
		resetHistoryBrowse();
		lastNonEmptyDraft = '';
		applyComposerText('');
	});

	$effect(() => {
		if (!autofocus || shellStore.focusedPane !== 'chat') return;
		void sessionId;
		void focusInput();
	});

	$effect(() => {
		if (historyIndex !== null && historyAppliedText !== null && value !== historyAppliedText) {
			// User edited while browsing — leave history mode.
			resetHistoryBrowse();
		}
		// While browsing history, do not treat recalled text as a live draft to stash.
		if (historyIndex !== null) return;

		const trimmed = value.trim();
		if (trimmed) {
			lastNonEmptyDraft = value;
			return;
		}
		if (skippingEmptyStash) return;
		if (!lastNonEmptyDraft.trim()) return;
		composerHistoryStore.stashUnsent(sessionId, {
			text: lastNonEmptyDraft,
			images: images.length > 0 ? images : undefined
		});
		lastNonEmptyDraft = '';
	});

	onDestroy(() => {
		if (value.trim() || images.length > 0) {
			composerHistoryStore.stashUnsent(sessionId, { text: value, images });
		}
		attachments.destroy();
	});

	async function submit() {
		const trimmed = value.trim();
		const action = slash.resolveSubmitAction(trimmed);
		if (action.kind === 'handled') return;
		if (!canSubmit || disabled || resolvingWebContext || !modelStore.selected) return;
		const filePaths = input?.getFilePaths() ?? [];
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
		inputController.sendTurn({
			text: action.text,
			displayText,
			images: images.length > 0 ? images : undefined,
			filePaths: filePaths.length > 0 ? filePaths : undefined,
			webContexts: webContexts.length > 0 ? webContexts : undefined
		});
		if (contextsBeforeResolve > 0) shellStore.clearWebContextForActive();
		skippingEmptyStash = true;
		input?.clear();
		clearDraft();
		void tick().then(() => {
			skippingEmptyStash = false;
		});
	}

	async function navigateHistory(direction: 'up' | 'down') {
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
			historyLiveDraft = value;
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
		const pending = composerHistoryStore.getPending(sessionId);
		const recallImages =
			pending?.text.trim() === text.trim() && pending.images?.length
				? pending.images
				: [];
		applyComposerText(text, recallImages);
	}

	function onKeydown(e: KeyboardEvent) {
		if (
			!e.isComposing &&
			matchesShortcut(e, settingsStore.settings.shortcuts.cycleReasoningEffort)
		) {
			e.preventDefault();
			cycleReasoningEffort();
			return;
		}
		if (slash.handleMenuKeydown(e)) return;
		if (mentions.handleMentionMenuKeydown(e)) return;
		if (
			!e.isComposing &&
			!e.metaKey &&
			!e.ctrlKey &&
			!e.altKey &&
			(e.key === 'Backspace' || e.key === 'Delete')
		) {
			const sel = window.getSelection();
			if (!sel || sel.isCollapsed) {
				const removal = nextAttachmentRemoval(value, images, pendingWebContexts.length);
				if (removal?.kind === 'image') {
					e.preventDefault();
					attachments.removeImage(removal.id);
					return;
				}
				if (removal?.kind === 'webContext') {
					e.preventDefault();
					shellStore.removeWebContextAt(removal.index);
					return;
				}
			}
		}
		if (
			!e.isComposing &&
			!e.metaKey &&
			!e.ctrlKey &&
			!e.altKey &&
			(e.key === 'ArrowUp' || e.key === 'ArrowDown')
		) {
			const inHistory = historyIndex !== null;
			const canStepUp = e.key === 'ArrowUp' && (input?.isCaretAtStart() ?? true);
			const canStepDown =
				e.key === 'ArrowDown' && inHistory && (input?.isCaretAtEnd() ?? true);
			if (canStepUp || canStepDown) {
				e.preventDefault();
				void navigateHistory(e.key === 'ArrowUp' ? 'up' : 'down');
				return;
			}
		}
		if (matchesShortcut(e, settingsStore.settings.shortcuts.stopResponse) && streaming) {
			const sel = window.getSelection();
			if (!sel || sel.isCollapsed) {
				e.preventDefault();
				onStop?.();
				return;
			}
		}
		if (!e.isComposing && matchesShortcut(e, settingsStore.settings.shortcuts.insertNewline)) {
			return;
		}
		if (!e.isComposing && matchesShortcut(e, settingsStore.settings.shortcuts.sendMessage)) {
			e.preventDefault();
			void submit();
		}
	}

	function removeQueued(id: string) {
		onRemoveQueued?.(id);
	}

	function cycleReasoningEffort() {
		const supported = modelStore.selected?.reasoningEffortOptions ?? [];
		if (supported.length === 0) return;
		const next = nextReasoningEffort(getReasoningEffort(sessionId), supported);
		setReasoningEffort(sessionId, next);
	}
</script>

<div
	class="composer"
	role="group"
	aria-label="Message composer"
	class:hero={variant === 'hero'}
	class:dragging={attachments.dragActive}
	ondragenter={attachments.onDragEnter}
	ondragover={attachments.onDragOver}
	ondragleave={attachments.onDragLeave}
	ondrop={attachments.onDrop}
>
	{#if attachments.dragActive}
		<div class="drop-overlay" aria-hidden="true">
			<FileText size={18} stroke-width={1.8} />
			<span
				>{attachments.dropProcessing
					? 'Reading files…'
					: 'Drop text files to add context'}</span
			>
		</div>
	{/if}

	{#if attachments.dropMessage}
		<div class="drop-message" role="status" transition:fade={{ duration: 120 }}>
			{attachments.dropMessage}
		</div>
	{/if}

	<ComposerSlashMenus {slash} bind:menuRef={skillMenu} />
	<ComposerMentionMenu {mentions} bind:menuRef={mentionMenu} />

	<MessageQueuePanel {queuedCount} {queuedMessages} onRemove={removeQueued} />

	{#if pendingContextRefs.length > 0}
		<MessageContextChips
			contexts={pendingContextRefs}
			removable
			onRemove={(index) => shellStore.removeWebContextAt(index)}
			onClearAll={() => shellStore.clearWebContextForActive()}
		/>
	{/if}

	<RichComposerInput
		bind:this={input}
		bind:value
		skillNames={slash.skillNames}
		mentionsEnabled={mentions.mentionsEnabled}
		caretTrail={settingsStore.settings.appearance.caretTrail}
		caretColor={settingsStore.settings.appearance.heroComposer.glowColor}
		onkeydown={onKeydown}
		placeholder={streaming
			? 'Add a follow-up…'
			: variant === 'hero'
				? heroPlaceholders[heroPlaceholderIndex]
				: 'Type something…'}
		onfiles={(files) => void attachments.addImageFiles(files)}
		onmentionquery={mentions.onMentionQuery}
	/>

	<ImageAttachments {images} onRemove={attachments.removeImage} />

	{#if images.length > 0 && modelStore.selected?.visionKnown && modelStore.selected?.vision === false}
		<p class="vision-capability-hint" role="status">
			This model can't view images — they'll be sent as metadata only.
		</p>
	{/if}

	<ComposerToolbar
		hasWorkspace={mentions.hasWorkspace}
		{currentWorkspaceLabel}
		workspaceMenuOpen={slash.workspaceMenuOpen}
		{contextWindowUsage}
		{streaming}
		{canSubmit}
		{disabled}
		{onModelChange}
		reasoningEffort={getReasoningEffort(sessionId)}
		reasoningEffortOptions={modelStore.selected?.reasoningEffortOptions ?? []}
		onCycleReasoningEffort={cycleReasoningEffort}
		onOpenChangeWorkspace={slash.openChangeWorkspace}
		{onStop}
		onSubmit={() => void submit()}
	/>
</div>

<style>
	.composer {
		position: relative;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		padding: 14px 14px 10px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
		transition:
			width var(--duration-flight) var(--ease-smooth),
			padding var(--duration-flight) var(--ease-smooth),
			border-radius var(--duration-flight) var(--ease-smooth),
			box-shadow var(--duration-flight) var(--ease-smooth),
			transform var(--duration-flight) var(--ease-smooth),
			background var(--duration-flight) var(--ease-smooth);
	}

	.composer.dragging {
		border-color: rgba(37, 99, 235, 0.26);
		background: #f8fbff;
		box-shadow:
			var(--shadow-card),
			0 0 0 4px rgba(37, 99, 235, 0.08);
	}

	.drop-overlay {
		position: absolute;
		inset: 8px;
		z-index: 20;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		border: 1px dashed rgba(37, 99, 235, 0.34);
		border-radius: calc(var(--radius-card) - 6px);
		background: rgba(255, 255, 255, 0.78);
		color: #1d4ed8;
		font-size: 13px;
		font-weight: 600;
		pointer-events: none;
		backdrop-filter: blur(10px);
		-webkit-backdrop-filter: blur(10px);
	}

	.drop-message {
		position: absolute;
		right: 12px;
		bottom: calc(100% + 8px);
		z-index: 25;
		max-width: min(360px, calc(100vw - 32px));
		padding: 7px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.96);
		box-shadow: var(--shadow-card);
		color: var(--text-muted);
		font-size: 12px;
		line-height: 1.35;
	}

	.vision-capability-hint {
		margin: 0;
		font-size: 12px;
		line-height: 1.35;
		color: var(--text-muted);
	}

	.composer.hero {
		padding: 24px 24px 16px;
		border-radius: 24px;
		box-shadow: 0 18px 60px rgba(15, 23, 42, 0.12);
	}
</style>
