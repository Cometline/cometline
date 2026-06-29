<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import { fade } from 'svelte/transition';
	import { FileText } from '@lucide/svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
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
	import { chatStore } from '$lib/stores/chat.svelte';
	import {
		estimateChatContextTokens,
		estimateTokensFromText,
		resolveContextWindow
	} from '$lib/context-window';
	import { workspaceLabel } from '$lib/sessions/group-by-workspace';
	import type { ImageAttachment } from '$lib/types';
	import type { ComposerInputRef } from '$lib/components/composer/composer-input-ref';
	import { createComposerInputController } from '$lib/components/composer/composer-controller.svelte';
	import { createComposerAttachmentsController } from '$lib/components/composer/composer-attachments.svelte';
	import { createComposerMentionsController } from '$lib/components/composer/composer-mentions.svelte';
	import { createComposerSlashController } from '$lib/components/composer/composer-slash.svelte';

	let {
		onSend,
		onLocalUserMessage,
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
		onLocalUserMessage?: (text: string) => void;
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

	function clearDraft() {
		value = '';
		images = [];
	}

	const getInput = (): ComposerInputRef | null => input;

	const inputController = createComposerInputController({
		onSend: (payload) => onSend(payload),
		getValue: () => value,
		getImages: () => images,
		getDisabled: () => disabled,
		getHasSelectedModel: () => Boolean(modelStore.selected),
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
		onLocalUserMessage: (text) => onLocalUserMessage?.(text),
		onModelChange: (option) => onModelChange?.(option),
		onWorkspaceChanged: () => onWorkspaceChanged?.(),
		onTranscriptCleared: () => onTranscriptCleared?.(),
		setDropMessage: (message) => attachments.setDropMessage(message),
		focusInput,
		getSkillMenuRef: () => skillMenu
	});

	const canSubmit = $derived(inputController.canSubmit());
	const contextWindowUsage = $derived.by(() => {
		const limit = resolveContextWindow(settingsStore.settings.cometmind.contextWindowLimit);
		const items = sessionId && chatStore.sessionID === sessionId ? chatStore.items : [];
		const draftTokens = value.trim() ? estimateTokensFromText(value) : 0;
		const used = estimateChatContextTokens(items) + draftTokens;
		return { used, limit };
	});
	const currentWorkspaceLabel = $derived(
		mentions.hasWorkspace ? workspaceLabel(shellStore.workspacePath) : ''
	);

	export function focus() {
		void focusInput();
	}

	$effect(() => {
		if (!autofocus) return;
		void sessionId;
		void focusInput();
	});

	onDestroy(() => attachments.destroy());

	function submit() {
		const trimmed = value.trim();
		const action = slash.resolveSubmitAction(trimmed);
		if (action.kind === 'handled') return;
		if (!canSubmit || disabled || !modelStore.selected) return;
		const filePaths = input?.getFilePaths() ?? [];
		inputController.sendTurn({
			text: action.text,
			images: images.length > 0 ? images : undefined,
			filePaths: filePaths.length > 0 ? filePaths : undefined
		});
		input?.clear();
		clearDraft();
	}

	function onKeydown(e: KeyboardEvent) {
		if (slash.handleMenuKeydown(e)) return;
		if (mentions.handleMentionMenuKeydown(e)) return;
		if (matchesShortcut(e, settingsStore.settings.shortcuts.stopResponse) && streaming) {
			const sel = window.getSelection();
			if (!sel || sel.isCollapsed) {
				e.preventDefault();
				onStop?.();
				return;
			}
		}
		if (!e.isComposing && matchesShortcut(e, settingsStore.settings.shortcuts.insertNewline)) {
			e.preventDefault();
			input?.insertText('\n');
			return;
		}
		if (!e.isComposing && matchesShortcut(e, settingsStore.settings.shortcuts.sendMessage)) {
			e.preventDefault();
			submit();
		}
	}

	function removeQueued(id: string) {
		onRemoveQueued?.(id);
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

	<RichComposerInput
		bind:this={input}
		bind:value
		skillNames={slash.skillNames}
		mentionsEnabled={mentions.hasWorkspace}
		caretTrail={settingsStore.settings.appearance.caretTrail}
		caretColor={settingsStore.settings.appearance.heroComposer.glowColor}
		onkeydown={onKeydown}
		placeholder={streaming
			? 'Add a follow-up…'
			: variant === 'hero'
				? 'Type something. Anything.'
				: 'Type something…'}
		onfiles={(files) => void attachments.addImageFiles(files)}
		onmentionquery={mentions.onMentionQuery}
	/>

	<ImageAttachments {images} onRemove={attachments.removeImage} />

	<ComposerToolbar
		hasWorkspace={mentions.hasWorkspace}
		{currentWorkspaceLabel}
		workspaceMenuOpen={slash.workspaceMenuOpen}
		{contextWindowUsage}
		{streaming}
		{canSubmit}
		{disabled}
		{onModelChange}
		onOpenChangeWorkspace={slash.openChangeWorkspace}
		{onStop}
		onSubmit={submit}
	/>
</div>

<style>
	.composer {
		position: relative;
		background: rgba(255, 255, 255, 0.74);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		padding: 14px 14px 10px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		backdrop-filter: blur(18px) saturate(170%);
		-webkit-backdrop-filter: blur(18px) saturate(170%);
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
		background: rgba(248, 251, 255, 0.92);
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

	.composer.hero {
		padding: 24px 24px 16px;
		border-radius: 24px;
		box-shadow: 0 18px 60px rgba(15, 23, 42, 0.12);
	}
</style>
