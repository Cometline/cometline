<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import { Check, Send, Sparkles } from '@lucide/svelte';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';

	let {
		onSend,
		disabled = false,
		variant = 'dock'
	}: { onSend: (text: string) => void; disabled?: boolean; variant?: 'hero' | 'dock' } =
		$props();

	let value = $state('');
	let modelOpen = $state(false);
	let rows = $derived(Math.min(8, Math.max(3, value.split('\n').length)));

	function submit() {
		const text = value.trim();
		if (!text || disabled) return;
		onSend(text);
		value = '';
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			submit();
		}
	}

	function selectModel(option: ModelOption) {
		modelStore.select(option);
		settingsStore.setSelectedModel(option.model_id);
		modelOpen = false;
	}

	function closeModelMenu(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		const current = e.currentTarget as Node;
		if (next && current.contains(next)) return;
		modelOpen = false;
	}
</script>

<div class="composer" class:hero={variant === 'hero'}>
	<textarea
		bind:value
		{rows}
		placeholder={variant === 'hero' ? 'Type something. Anything.' : 'Type something…'}
		onkeydown={onKeydown}
		{disabled}
		aria-label="Message input"
	></textarea>

	<div class="composer-footer">
		<div class="composer-tools">
			<div class="model-picker" onfocusout={closeModelMenu}>
				<button
					class="model-button"
					aria-label="Select model"
					aria-expanded={modelOpen}
					title="Select model for new chats"
					onclick={() => (modelOpen = !modelOpen)}
				>
					<Sparkles size={14} stroke-width={1.8} />
					<span>{modelStore.selected.label}</span>
					<svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" aria-hidden="true">
						<path d="M2 4l3 3 3-3H2z" />
					</svg>
				</button>

				{#if modelOpen}
					<div class="model-menu" transition:fly={{ y: 6, duration: 120 }}>
						{#each modelStore.options as option (option.id)}
							<button class="model-option" onclick={() => selectModel(option)} transition:fade={{ duration: 90 }}>
								<span class="model-option-copy">
									<strong>{option.label}</strong>
									<small>{option.model_id} · {option.description}</small>
								</span>
								{#if option.id === modelStore.selected.id}
									<Check size={14} stroke-width={2} />
								{/if}
							</button>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<div class="composer-actions">
			<button class="send-button" onclick={submit} disabled={!value.trim() || disabled} aria-label="Send">
				<Send size={16} stroke-width={1.8} />
			</button>
		</div>
	</div>
</div>

<style>
	.composer {
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		padding: 14px 14px 10px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		transition:
			padding 280ms cubic-bezier(0.22, 1, 0.36, 1),
			border-radius 280ms cubic-bezier(0.22, 1, 0.36, 1),
			box-shadow 280ms cubic-bezier(0.22, 1, 0.36, 1),
			transform 280ms cubic-bezier(0.22, 1, 0.36, 1);
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
		gap: 6px;
		min-width: 0;
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
		width: 290px;
		padding: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.98);
		box-shadow: var(--shadow-card);
		z-index: 30;
	}

	.model-option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		width: 100%;
		padding: 9px 10px;
		text-align: left;
		color: var(--text-main);
	}

	.model-option-copy {
		display: flex;
		min-width: 0;
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
</style>
