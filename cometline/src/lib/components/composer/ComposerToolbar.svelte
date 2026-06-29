<script lang="ts">
	import { Folder, Send, Square } from '@lucide/svelte';
	import ContextWindowRing from '$lib/components/composer/ContextWindowRing.svelte';
	import ModelPicker from '$lib/components/composer/ModelPicker.svelte';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';

	let {
		hasWorkspace,
		currentWorkspaceLabel,
		workspaceMenuOpen,
		contextWindowUsage,
		streaming,
		canSubmit,
		disabled,
		onModelChange,
		onOpenChangeWorkspace,
		onStop,
		onSubmit
	}: {
		hasWorkspace: boolean;
		currentWorkspaceLabel: string;
		workspaceMenuOpen: boolean;
		contextWindowUsage: { used: number; limit: number };
		streaming: boolean;
		canSubmit: boolean;
		disabled: boolean;
		onModelChange?: (option: ModelOption) => void | Promise<void>;
		onOpenChangeWorkspace: () => void;
		onStop?: () => void;
		onSubmit: () => void;
	} = $props();
</script>

<div class="composer-footer">
	<div class="composer-tools">
		<ModelPicker {onModelChange} />
		{#if hasWorkspace}
			<button
				type="button"
				class="workspace-indicator"
				title={shellStore.workspacePath}
				aria-label="Change workspace"
				aria-expanded={workspaceMenuOpen}
				onclick={onOpenChangeWorkspace}
			>
				<Folder size={14} stroke-width={1.8} />
				<span>{currentWorkspaceLabel}</span>
			</button>
		{/if}
	</div>

	<div class="composer-actions">
		{#if contextWindowUsage}
			<ContextWindowRing
				usedTokens={contextWindowUsage.used}
				limitTokens={contextWindowUsage.limit}
			/>
		{/if}
		{#if streaming}
			<button class="stop-button" onclick={() => onStop?.()} aria-label="Stop response">
				<Square size={14} fill="currentColor" stroke-width={0} />
			</button>
		{/if}
		<button
			class="send-button"
			onclick={onSubmit}
			disabled={!canSubmit || disabled || !modelStore.selected}
			aria-label={streaming ? 'Queue follow-up' : 'Send'}
		>
			<Send size={16} stroke-width={1.8} />
		</button>
	</div>
</div>

<style>
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
		border-radius: 999px;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 58%,
			var(--accent, #0066cc)
		) !important;
		transition:
			color 160ms ease,
			background 160ms ease,
			box-shadow 160ms ease;
	}

	.send-button:hover:not(:disabled) {
		color: var(--hero-composer-glow-color, #72c0ff) !important;
		background: var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24)) !important;
		box-shadow: 0 0 14px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.send-button:active:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 22%,
			transparent
		) !important;
		box-shadow: 0 0 8px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.stop-button {
		display: grid;
		place-items: center;
		padding: 6px;
		border-radius: 999px;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 58%,
			var(--accent, #0066cc)
		) !important;
		transition:
			color 160ms ease,
			background 160ms ease,
			box-shadow 160ms ease;
	}

	.stop-button:hover:not(:disabled) {
		color: var(--hero-composer-glow-color, #72c0ff) !important;
		background: var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24)) !important;
		box-shadow: 0 0 14px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.stop-button:active:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 22%,
			transparent
		) !important;
		box-shadow: 0 0 8px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.workspace-indicator {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		max-width: 100%;
		padding: 5px 8px;
		font-size: 13px;
		font-weight: 500;
		line-height: 1;
		color: var(--text-muted);
		white-space: nowrap;
		border: none;
		background: transparent;
		border-radius: 7px;
		cursor: pointer;
	}

	.workspace-indicator span {
		min-width: 0;
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
		text-transform: uppercase;
	}

	.workspace-indicator :global(svg) {
		flex-shrink: 0;
	}
</style>
