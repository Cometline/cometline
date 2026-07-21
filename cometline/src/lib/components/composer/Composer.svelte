<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import { fade } from 'svelte/transition';
	import { FileText } from '@lucide/svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import type { ChatTurnPayload, WebContext } from '$lib/actions/start-chat';
	import type { PendingWebContext } from '$lib/stores/shell.svelte';
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
	let resolvingWebContext = $state(false);
	const heroPlaceholders = [
		'Type something. Anything.',
		'Ask a question.',
		'Share a thought.',
		'Drop in a task.',
		'Bring an idea to life.'
	];
	let heroPlaceholderIndex = $state(0);

	onMount(() => {
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
	const pendingWebContexts = $derived(shellStore.pendingWebContexts);

	export function focus() {
		void focusInput();
	}

	$effect(() => {
		if (!autofocus || shellStore.focusedPane !== 'chat') return;
		void sessionId;
		void focusInput();
	});

	onDestroy(() => attachments.destroy());

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

	function webContextLabel(context: PendingWebContext): string {
		if (context.kind === 'file' && 'role' in context && context.role === 'viewing') {
			const name = context.title?.trim() || fileNameFromSource(context.source);
			return `Viewing ${name}`;
		}
		const title = context.title?.trim();
		if (title) return title;
		if (context.kind === 'terminal') return 'Terminal selection';
		if (context.kind === 'file') return fileNameFromSource(context.source);
		return pageNameFromSource(context.source);
	}

	function fileNameFromSource(source: string): string {
		if (source.startsWith('@runtime/wiki/')) {
			const path = source.slice('@runtime/wiki/'.length);
			return path.split(/[/\\]/).filter(Boolean).pop() || path || 'Wiki';
		}
		const path = source.replace(/^workspace-file:/, '');
		return path.split(/[/\\]/).filter(Boolean).pop() || path || 'File';
	}

	function pageNameFromSource(source: string): string {
		try {
			const url = new URL(source);
			return url.hostname || source;
		} catch {
			return source || 'Page';
		}
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

	{#if pendingWebContexts.length > 0}
		<div class="web-context-chips" role="list" aria-label="Chat context">
			{#each pendingWebContexts as context, index (context.source + ':' + index)}
				<div class="web-context-chip" role="listitem">
					<FileText size={14} />
					<span title={webContextLabel(context)}>{webContextLabel(context)}</span>
					<button
						type="button"
						onclick={() => shellStore.removeWebContextAt(index)}
						aria-label="Remove {webContextLabel(context)}"
					>
						×
					</button>
				</div>
			{/each}
			{#if pendingWebContexts.length > 1}
				<button
					type="button"
					class="web-context-clear"
					onclick={() => shellStore.clearWebContextForActive()}
				>
					Clear all
				</button>
			{/if}
		</div>
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

	.web-context-chips {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px;
	}

	.web-context-chip {
		display: flex;
		align-items: center;
		gap: 7px;
		max-width: 100%;
		padding: 7px 9px;
		border: 1px solid
			color-mix(in srgb, var(--hero-composer-glow-color, #72c0ff) 20%, var(--border-soft));
		border-radius: 9px;
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 7%,
			var(--panel-bg)
		);
		color: var(--text-muted);
		font-size: 12px;
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.58);
	}

	.web-context-clear {
		border: 0;
		background: transparent;
		color: var(--text-soft);
		font-size: 12px;
		cursor: pointer;
		padding: 4px 6px;
	}

	.web-context-clear:hover {
		color: var(--text-main);
	}

	.web-context-chip :global(svg) {
		flex-shrink: 0;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 62%,
			var(--accent, #0066cc)
		);
	}

	.web-context-chip span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.web-context-chip button {
		margin-left: auto;
		border: 0;
		background: transparent;
		color: var(--text-soft);
		font-size: 18px;
		line-height: 1;
		cursor: pointer;
	}

	.web-context-chip button:hover {
		color: var(--text-main);
	}

	.composer.hero {
		padding: 24px 24px 16px;
		border-radius: 24px;
		box-shadow: 0 18px 60px rgba(15, 23, 42, 0.12);
	}
</style>
