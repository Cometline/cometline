<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import RichComposerInput from '$lib/features/composer/components/RichComposerInput.svelte';
	import ImageAttachments from '$lib/features/composer/components/ImageAttachments.svelte';
	import MessageQueuePanel from '$lib/features/composer/components/MessageQueuePanel.svelte';
	import ComposerSlashMenus from '$lib/features/composer/components/ComposerSlashMenus.svelte';
	import ComposerMentionMenu from '$lib/features/composer/components/ComposerMentionMenu.svelte';
	import ComposerToolbar from '$lib/features/composer/components/ComposerToolbar.svelte';
	import ComposerDropFeedback from '$lib/features/composer/components/composer/ComposerDropFeedback.svelte';
	import MessageContextChips from '$lib/features/chat/components/MessageContextChips.svelte';
	import { messageContextRefsFromPending } from '$lib/features/chat/message-context';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { composerHistoryStore } from '$lib/stores/composer-history.svelte';
	import { DEFAULT_CONTEXT_WINDOW_LIMIT, resolveContextWindowUsage } from '$lib/context-window';
	import { workspaceLabel } from '$lib/sessions/group-by-workspace';
	import type { ImageAttachment } from '$lib/types';
	import type { ComposerInputRef } from '$lib/features/composer/composer-input-ref';
	import { createComposerInputController } from '$lib/features/composer/composer-controller.svelte';
	import { createComposerAttachmentsController } from '$lib/features/composer/composer-attachments.svelte';
	import { createComposerMentionsController } from '$lib/features/composer/composer-mentions.svelte';
	import { createComposerSlashController } from '$lib/features/composer/composer-slash.svelte';
	import { createComposerAgentModeController } from '$lib/features/composer/composer-agent-mode.svelte';
	import { createComposerDraftHistoryController } from '$lib/features/composer/composer-draft-history.svelte';
	import type { PendingUnsentDraft } from '$lib/features/composer/composer-history';
	import { createComposerTurnController } from '$lib/features/composer/composer-turn.svelte';
	import { getReasoningEffort } from '$lib/stores/reasoning-effort.svelte';

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
		variant = 'dock'
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
	} = $props();

	let value = $state('');
	let images = $state<ImageAttachment[]>([]);
	let input = $state<RichComposerInput | null>(null);
	let skillMenu = $state<HTMLDivElement | null>(null);
	let mentionMenu = $state<HTMLDivElement | null>(null);
	const agentModes = createComposerAgentModeController({ getSessionId: () => sessionId });
	const agentMode = $derived(agentModes.agentMode);
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
	});

	$effect(() => {
		if (variant !== 'hero') return;
		const rotation = window.setInterval(() => {
			heroPlaceholderIndex = (heroPlaceholderIndex + 1) % heroPlaceholders.length;
		}, 10000);

		return () => window.clearInterval(rotation);
	});

	function clearDraft() {
		value = '';
		images = [];
	}

	const getInput = (): ComposerInputRef | null => input;
	const setImages = (next: ImageAttachment[]) => {
		images = next;
	};

	const draftHistory = createComposerDraftHistoryController({
		getValue: () => value,
		setValue: (next) => {
			value = next;
		},
		getImages: () => images,
		setImages,
		getInput,
		getSessionId: () => sessionId,
		onSessionChanged: () => agentModes.resetStoreBinding()
	});

	const inputController = createComposerInputController({
		onSend: (payload) => {
			draftHistory.recordSentHistory(payload);
			onSend(payload);
		},
		getValue: () => value,
		getImages: () => images,
		getDisabled: () => disabled,
		getHasSelectedModel: () => Boolean(modelStore.selected),
		getReasoningEffort: () => getReasoningEffort(sessionId),
		getReasoningEffortOptions: () => modelStore.selected?.reasoningEffortOptions ?? [],
		getAgentMode: () => agentMode,
		clearDraft,
		applyDraft: (draft) => draftHistory.applyComposerText(draft.text, draft.images ?? [])
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

	const turn = createComposerTurnController({
		getValue: () => value,
		getImages: () => images,
		getInput,
		getSessionId: () => sessionId,
		getDisabled: () => disabled,
		getStreaming: () => streaming,
		getCanSubmit: () => inputController.canSubmit(),
		getSlash: () => slash,
		getMentionKeydown: () => mentions.handleMentionMenuKeydown,
		cycleAgentMode: agentModes.cycleAgentMode,
		isBrowsingHistory: () => draftHistory.browsing,
		navigateHistory: draftHistory.navigateHistory,
		sendTurn: (payload) => inputController.sendTurn(payload),
		clearDraft,
		skipEmptyStash: draftHistory.skipEmptyStash,
		removeImage: attachments.removeImage,
		onStop: () => onStop?.(),
		onModelChange: (option) => onModelChange?.(option)
	});

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
		onModelChange: (option) => turn.changeModel(option),
		onWorkspaceChanged: () => onWorkspaceChanged?.(),
		onTranscriptCleared: () => onTranscriptCleared?.(),
		setDropMessage: (message) => attachments.setDropMessage(message),
		focusInput,
		getSkillMenuRef: () => skillMenu
	});

	const canSubmit = $derived(inputController.canSubmit());
	const contextWindowUsage = $derived.by(() => {
		const items = sessionId && chatStore.sessionID === sessionId ? chatStore.items : [];
		const budget =
			sessionId && chatStore.sessionID === sessionId ? chatStore.contextBudget : null;
		const selected = modelStore.selected;
		return resolveContextWindowUsage({
			budget,
			items,
			draftText: value,
			contextWindow: selected?.context ?? DEFAULT_CONTEXT_WINDOW_LIMIT,
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

	export function restoreDraft(draft: PendingUnsentDraft) {
		if (!inputController.restoreDraft(draft)) return false;
		draftHistory.markDraftRestored(draft.text);
		void focusInput({ position: 'end' });
		return true;
	}

	$effect(() => {
		draftHistory.trackSessionChange();
	});

	$effect(() => {
		agentModes.bindFromStore();
	});

	$effect(() => {
		draftHistory.syncDraftStash();
	});

	onDestroy(() => {
		draftHistory.stashUnsentNow();
		attachments.destroy();
	});

	function removeQueued(id: string) {
		onRemoveQueued?.(id);
	}
</script>

<div
	class="composer"
	class:hero={variant === 'hero'}
	class:plan={agentMode === 'plan'}
	class:dragging={attachments.dragActive}
	role="group"
	aria-label="Message composer"
	ondragenter={attachments.onDragEnter}
	ondragover={attachments.onDragOver}
	ondragleave={attachments.onDragLeave}
	ondrop={attachments.onDrop}
>
	<div class="sr-only" role="status" aria-live="polite">{agentModes.modeAnnouncement}</div>
	<ComposerDropFeedback
		dragActive={attachments.dragActive}
		dropProcessing={attachments.dropProcessing}
		dropMessage={attachments.dropMessage}
	/>

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

	<ImageAttachments {images} onRemove={attachments.removeImage} />

	<RichComposerInput
		bind:this={input}
		bind:value
		skillNames={slash.skillNames}
		mentionsEnabled={mentions.mentionsEnabled}
		caretTrail={settingsStore.settings.appearance.caretTrail}
		caretColor={settingsStore.settings.appearance.heroComposer.glowColor}
		onkeydown={turn.onKeydown}
		placeholder={streaming
			? 'Add a follow-up…'
			: variant === 'hero'
				? heroPlaceholders[heroPlaceholderIndex]
				: 'Type something…'}
		onfiles={(files) => void attachments.addImageFiles(files)}
		onmentionquery={mentions.onMentionQuery}
	/>

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
		onModelChange={(option) => turn.changeModel(option)}
		reasoningEffort={turn.currentReasoningEffort()}
		reasoningEffortOptions={modelStore.selected?.reasoningEffortOptions ?? []}
		onCycleReasoningEffort={turn.cycleReasoningEffort}
		{agentMode}
		agentModeKnown={agentModes.agentModeKnown}
		onSwitchToAuto={() => agentModes.setAgentMode('auto')}
		onOpenChangeWorkspace={slash.openChangeWorkspace}
		{onStop}
		onSubmit={() => void turn.submit()}
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
		background: var(--color-f8fbff);
		box-shadow:
			var(--shadow-card),
			0 0 0 4px rgba(37, 99, 235, 0.08);
	}

	.composer.plan {
		border-color: var(--plan-border);
		box-shadow:
			var(--shadow-card),
			0 0 0 3px var(--plan-ring);
	}

	.composer.plan.hero {
		box-shadow:
			0 18px 60px rgba(15, 23, 42, 0.12),
			0 0 0 3px var(--plan-ring);
	}

	/* Drag feedback stays visible over plan styling. */
	.composer.plan.dragging {
		border-color: rgba(37, 99, 235, 0.26);
		box-shadow:
			var(--shadow-card),
			0 0 0 4px rgba(37, 99, 235, 0.08);
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
