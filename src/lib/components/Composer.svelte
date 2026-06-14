<script lang="ts">
	import { tick } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { Check, ChevronDown, Send, Sparkles, Square, X } from '@lucide/svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';

	let {
		onSend,
		onStop,
		onRemoveQueued,
		disabled = false,
		streaming = false,
		queuedCount = 0,
		queuedMessages = [],
		waitingForReply = false,
		turnProcessing = false,
		variant = 'dock',
		autofocus = true
	}: {
		onSend: (text: string) => void;
		onStop?: () => void;
		onRemoveQueued?: (id: string) => void;
		disabled?: boolean;
		streaming?: boolean;
		queuedCount?: number;
		queuedMessages?: QueuedMessage[];
		waitingForReply?: boolean;
		turnProcessing?: boolean;
		variant?: 'hero' | 'dock';
		autofocus?: boolean;
	} = $props();

	let value = $state('');
	let textarea = $state<HTMLTextAreaElement | null>(null);
	let modelOpen = $state(false);
	let modelSearch = $state('');
	let queuePreviewOpen = $state(false);
	let queuePicker = $state<HTMLDivElement | null>(null);
	let sendLocked = $derived(turnProcessing && !streaming);
	let rows = $derived(Math.min(8, Math.max(3, value.split('\n').length)));
	let filteredModelOptions = $derived.by(() => {
		const query = modelSearch.trim().toLowerCase();
		if (!query) return modelStore.options;
		return modelStore.options.filter(
			(option) =>
				option.label.toLowerCase().includes(query) ||
				option.modelId.toLowerCase().includes(query) ||
				option.providerName.toLowerCase().includes(query)
		);
	});
	let groupedModelOptions = $derived.by(() => {
		const groups: {
			providerId: string;
			providerName: string;
			providerMethod: string;
			options: ModelOption[];
		}[] = [];
		for (const option of filteredModelOptions) {
			let group = groups.find((item) => item.providerId === option.providerId);
			if (!group) {
				group = {
					providerId: option.providerId,
					providerName: option.providerName,
					providerMethod: option.providerMethod,
					options: []
				};
				groups.push(group);
			}
			group.options.push(option);
		}
		return groups;
	});

	$effect(() => {
		if (!autofocus) return;
		void focusInput();
	});

	$effect(() => {
		if (queuedCount === 0) queuePreviewOpen = false;
	});

	$effect(() => {
		if (!queuePreviewOpen) return;
		function onPointerDown(e: PointerEvent) {
			if (queuePicker?.contains(e.target as Node)) return;
			queuePreviewOpen = false;
		}
		document.addEventListener('pointerdown', onPointerDown);
		return () => document.removeEventListener('pointerdown', onPointerDown);
	});

	function submit() {
		const text = value.trim();
		if (!text || disabled || sendLocked || !modelStore.selected) return;
		onSend(text);
		value = '';
	}

	function onKeydown(e: KeyboardEvent) {
		if (matchesShortcut(e, settingsStore.settings.shortcuts.stopResponse) && streaming) {
			const textarea = e.currentTarget as HTMLTextAreaElement;
			if (textarea.selectionStart === textarea.selectionEnd) {
				e.preventDefault();
				onStop?.();
				return;
			}
		}
		if (matchesShortcut(e, settingsStore.settings.shortcuts.sendMessage)) {
			e.preventDefault();
			submit();
		}
	}

	function selectModel(option: ModelOption) {
		modelStore.select(option);
		modelOpen = false;
		modelSearch = '';
	}

	function closeModelMenu(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		const current = e.currentTarget as Node;
		if (next && current.contains(next)) return;
		modelOpen = false;
		modelSearch = '';
	}

	function toggleQueuePreview() {
		queuePreviewOpen = !queuePreviewOpen;
	}

	function removeQueued(id: string) {
		onRemoveQueued?.(id);
	}

	async function focusInput() {
		await tick();
		textarea?.focus({ preventScroll: true });
	}
</script>

<div class="composer" class:hero={variant === 'hero'}>
	{#if queuedCount > 0}
		<div
			class="queue-picker"
			bind:this={queuePicker}
			in:fly={{ y: 4, duration: 140 }}
			out:fly={{ y: 4, duration: 120 }}
		>
			<button
				type="button"
				class="queue-banner"
				class:open={queuePreviewOpen}
				aria-live="polite"
				aria-expanded={queuePreviewOpen}
				aria-controls="queue-preview-panel"
				onclick={toggleQueuePreview}
			>
				<span>{queuedCount} {queuedCount === 1 ? 'message' : 'messages'} queued</span>
				<ChevronDown size={12} class={queuePreviewOpen ? 'expanded' : ''} />
			</button>

			{#if queuePreviewOpen}
				<div
					id="queue-preview-panel"
					class="queue-preview"
					role="region"
					aria-label="Queued messages"
					transition:fly={{ y: -4, duration: 120 }}
				>
					<ul class="queue-preview-list">
						{#each queuedMessages as message, index (message.id)}
							<li class="queue-preview-item">
								<span class="queue-preview-index">{index + 1}</span>
								<p class="queue-preview-text">{message.text}</p>
								<button
									type="button"
									class="queue-remove"
									aria-label={`Remove queued message ${index + 1}`}
									onpointerdown={(e) => {
										e.preventDefault();
										e.stopPropagation();
										removeQueued(message.id);
									}}
								>
									<X size={12} stroke-width={2} />
								</button>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		</div>
	{/if}

	<textarea
		bind:this={textarea}
		bind:value
		{rows}
		placeholder={waitingForReply
			? 'Waiting for reply…'
			: variant === 'hero'
				? 'Type something. Anything.'
				: 'Type something…'}
		onkeydown={onKeydown}
		aria-label="Message input"
	></textarea>

	<div class="composer-footer">
		<div class="composer-tools">
			<div class="model-picker" onfocusout={closeModelMenu}>
				<button
					class="model-button"
					aria-label="Select model"
					aria-expanded={modelOpen}
					title={modelStore.options.length > 0
						? 'Select model for new chats'
						: 'Enable a model in Settings first'}
					disabled={modelStore.options.length === 0}
					onclick={() => (modelOpen = !modelOpen)}
				>
					<Sparkles size={14} stroke-width={1.8} />
					<span>{modelStore.selected?.label ?? 'No enabled models'}</span>
					<svg
						width="10"
						height="10"
						viewBox="0 0 10 10"
						fill="currentColor"
						aria-hidden="true"
					>
						<path d="M2 4l3 3 3-3H2z" />
					</svg>
				</button>

				{#if modelOpen}
					<div class="model-menu" transition:fly={{ y: 6, duration: 120 }}>
						<input
							class="model-search"
							bind:value={modelSearch}
							placeholder="Search models..."
							spellcheck="false"
						/>
						{#each groupedModelOptions as group (group.providerId)}
							<div class="model-group" transition:fade={{ duration: 90 }}>
								<div class="model-group-heading">
									<strong>{group.providerName}</strong>
									<small>{group.providerMethod}</small>
								</div>
								{#each group.options as option (option.id)}
									<button
										class="model-option"
										onclick={() => selectModel(option)}
									>
										<span class="model-check">
											{#if option.id === modelStore.selected?.id}<Check
													size={14}
													stroke-width={2}
												/>{/if}
										</span>
										<span class="model-option-copy">
											<strong>{option.label}</strong>
											<small>{option.modelId}</small>
										</span>
									</button>
								{/each}
							</div>
						{:else}
							<p class="model-empty">No enabled models match your search.</p>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<div class="composer-actions">
			{#if streaming}
				<button class="stop-button" onclick={() => onStop?.()} aria-label="Stop response">
					<Square size={14} fill="currentColor" stroke-width={0} />
				</button>
			{:else}
				<button
					class="send-button"
					onclick={submit}
					disabled={!value.trim() || disabled || sendLocked || !modelStore.selected}
					aria-label="Send"
				>
					<Send size={16} stroke-width={1.8} />
				</button>
			{/if}
		</div>
	</div>
</div>

<style>
	.composer {
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

	.composer.hero {
		padding: 24px 24px 16px;
		border-radius: 24px;
		box-shadow: 0 18px 60px rgba(15, 23, 42, 0.12);
	}

	textarea {
		width: 100%;
		resize: none;
		border: none;
		background: transparent;
		font-size: 15px;
		line-height: 1.5;
		color: var(--text-main);
		outline: none;
		padding: 0;
		font-family: inherit;
	}

	textarea::placeholder {
		color: var(--text-soft);
	}

	.composer-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.composer-tools,
	.composer-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.queue-picker {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.queue-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		margin: -2px 0 -4px;
		padding: 6px 10px;
		border: none;
		border-radius: 10px;
		background: rgba(15, 23, 42, 0.04);
		font-size: 11px;
		font-weight: 500;
		line-height: 1.2;
		color: var(--text-soft);
		cursor: pointer;
		text-align: left;
	}

	.queue-banner:hover,
	.queue-banner.open {
		background: rgba(15, 23, 42, 0.07);
		color: var(--text-muted);
	}

	.queue-banner :global(svg) {
		flex-shrink: 0;
		transition: transform var(--duration-fast) var(--ease-smooth);
	}

	.queue-banner :global(.expanded) {
		transform: rotate(180deg);
	}

	.queue-preview {
		padding: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.92);
		box-shadow: var(--shadow-card);
	}

	.queue-preview-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-height: 160px;
		overflow-y: auto;
		scrollbar-gutter: stable;
	}

	.queue-preview-item {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 7px 8px;
		border-radius: 8px;
		background: rgba(15, 23, 42, 0.03);
	}

	.queue-remove {
		flex: 0 0 auto;
		display: grid;
		place-items: center;
		margin-left: auto;
		padding: 4px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-soft);
		cursor: pointer;
	}

	.queue-remove:hover {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.queue-preview-index {
		flex: 0 0 auto;
		font-size: 10px;
		font-weight: 600;
		line-height: 1.45;
		color: var(--text-soft);
	}

	.queue-preview-text {
		margin: 0;
		min-width: 0;
		flex: 1;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
		white-space: pre-wrap;
		word-break: break-word;
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		overflow: hidden;
	}

	.composer-footer button {
		border: none;
		background: transparent;
		color: var(--text-muted);
		border-radius: 7px;
		font-size: 13px;
		cursor: pointer;
	}

	.composer-footer button:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.composer-footer button:active:not(:disabled) {
		background: rgba(0, 0, 0, 0.07);
	}

	.composer-footer button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.send-button {
		display: grid;
		place-items: center;
		padding: 6px;
		color: var(--accent) !important;
	}

	.stop-button {
		display: grid;
		place-items: center;
		padding: 6px;
		color: var(--text-muted) !important;
	}

	.stop-button:hover:not(:disabled) {
		color: #b42318 !important;
		background: rgba(180, 35, 24, 0.08);
	}

	.model-picker {
		position: relative;
		min-width: 0;
	}

	.model-button {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		max-width: 100%;
		padding: 5px 8px;
		font-weight: 500;
		line-height: 1;
		white-space: nowrap;
	}

	.model-button span {
		min-width: 0;
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.model-button svg:last-child {
		flex-shrink: 0;
	}

	.model-menu {
		position: absolute;
		left: 0;
		bottom: calc(100% + 8px);
		width: 330px;
		max-height: 420px;
		overflow: auto;
		scrollbar-gutter: stable;
		padding: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(246, 249, 252, 0.98);
		box-shadow: var(--shadow-card);
		z-index: 30;
	}

	.model-search {
		width: 100%;
		border: none;
		border-bottom: 1px solid var(--border-soft);
		background: transparent;
		padding: 10px 12px 12px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		outline: none;
	}

	.model-search::placeholder {
		color: var(--text-soft);
	}

	.model-group {
		padding: 6px 0;
	}

	.model-group-heading {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		color: var(--text-muted);
	}

	.model-group-heading strong {
		font-size: 12px;
		font-weight: 700;
	}

	.model-group-heading small {
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		padding: 2px 7px;
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.02em;
	}

	.model-option {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		padding: 9px 10px;
		text-align: left;
		color: var(--text-main);
	}

	.model-check {
		width: 18px;
		display: grid;
		place-items: center;
		color: var(--text-main);
		flex-shrink: 0;
	}

	.model-option-copy {
		display: flex;
		min-width: 0;
		width: 100%;
		flex-direction: column;
		gap: 2px;
	}

	.model-option-copy strong,
	.model-option-copy small {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.model-option-copy strong {
		font-size: 13px;
		font-weight: 600;
	}

	.model-option-copy small {
		font-size: 11px;
		font-weight: 400;
		color: var(--text-soft);
	}

	.model-empty {
		margin: 0;
		padding: 12px;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
