<script lang="ts">
	import { Brain, Folder } from '@lucide/svelte';
	import ModelPicker from '#lib/features/composer/components/ModelPicker.svelte';
	import ComposerToolbarActions from '#lib/features/composer/components/composer/ComposerToolbarActions.svelte';
	import Tooltip from '#lib/components/Tooltip.svelte';
	import type { ModelOption } from '#lib/stores/model.svelte.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import type { AgentMode } from '#lib/types.js';

	let {
		hasWorkspace,
		currentWorkspaceLabel,
		workspaceMenuOpen,
		contextWindowUsage,
		streaming,
		canSubmit,
		disabled,
		onModelChange,
		reasoningEffort,
		reasoningEffortOptions,
		onCycleReasoningEffort,
		agentMode,
		agentModeKnown,
		onSwitchToAuto,
		onOpenChangeWorkspace,
		onStop,
		onSubmit
	}: {
		hasWorkspace: boolean;
		currentWorkspaceLabel: string;
		workspaceMenuOpen: boolean;
		contextWindowUsage: { used: number; limit: number; source: 'server' | 'fallback' };
		streaming: boolean;
		canSubmit: boolean;
		disabled: boolean;
		onModelChange?: (option: ModelOption) => void | Promise<void>;
		reasoningEffort: string;
		reasoningEffortOptions: string[];
		onCycleReasoningEffort: () => void;
		agentMode: AgentMode;
		agentModeKnown: boolean;
		onSwitchToAuto: () => void | Promise<void>;
		onOpenChangeWorkspace: () => void;
		onStop?: () => void;
		onSubmit: () => void;
	} = $props();

	const effortSupported = $derived(reasoningEffortOptions.length > 0);
	const effortLabel = $derived(
		reasoningEffort
			? reasoningEffort.charAt(0).toUpperCase() + reasoningEffort.slice(1)
			: 'Auto'
	);
</script>

<div class="composer-footer">
	<div class="composer-tools">
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
		<ModelPicker {onModelChange} />
		<Tooltip
			label={effortSupported
				? `Reasoning effort: ${effortLabel}`
				: 'Reasoning effort unavailable for this model'}
			action="cycleReasoningEffort"
			disabled={!effortSupported}
		>
			<button
				type="button"
				class="effort-button"
				class:active={Boolean(reasoningEffort)}
				disabled={!effortSupported}
				onclick={onCycleReasoningEffort}
				aria-label={effortSupported
					? `Reasoning effort: ${effortLabel}. Cycle effort.`
					: 'Reasoning effort unavailable for this model'}
			>
				<Brain size={15} stroke-width={1.8} />
				<span class="effort-label">{effortLabel}</span>
			</button>
		</Tooltip>
	</div>

	<ComposerToolbarActions
		{contextWindowUsage}
		{streaming}
		{canSubmit}
		{disabled}
		{agentMode}
		{agentModeKnown}
		{onSwitchToAuto}
		{onStop}
		{onSubmit}
	/>
</div>

<style>
	.composer-footer {
		position: relative;
		container-type: inline-size;
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.composer-tools {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		flex: 1 1 auto;
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

	.effort-button {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 5px 6px;
		line-height: 1.25;
	}

	.effort-button.active {
		color: var(--text-muted);
	}

	.effort-button :global(svg) {
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--color-72c0ff)) 58%,
			var(--accent, var(--accent))
		);
		transition:
			color 160ms ease,
			filter 160ms ease;
	}

	.effort-button.active :global(svg) {
		color: var(--hero-composer-glow-color, var(--color-72c0ff));
		filter: drop-shadow(0 0 5px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.2)));
	}

	.effort-label {
		max-width: 5.5rem;
		overflow: hidden;
		font-size: 11px;
		font-weight: 600;
		line-height: 1.25;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.workspace-indicator {
		display: inline-flex;
		flex: 0 1 auto;
		align-items: center;
		gap: 5px;
		min-width: 0;
		max-width: min(10rem, 42%);
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
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-transform: uppercase;
	}

	.workspace-indicator :global(svg) {
		flex-shrink: 0;
	}
</style>
