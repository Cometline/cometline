<script lang="ts">
	import { Brain, Folder, Send, Square } from '@lucide/svelte';
	import ContextWindowRing from '$lib/components/composer/ContextWindowRing.svelte';
	import ModelPicker from '$lib/components/composer/ModelPicker.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import type { AgentMode } from '$lib/types';

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

	const sendLabel = $derived(streaming ? 'Queue follow-up' : 'Send');
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

	<div class="composer-actions">
		{#if agentMode === 'plan' && agentModeKnown}
			<Tooltip label="Plan mode: read-only. Press Tab to switch to Auto.">
				<button
					type="button"
					class="plan-chip"
					onclick={() => void onSwitchToAuto()}
					aria-label="Plan mode: read-only. Click to switch to Auto."
				>
					plan
				</button>
			</Tooltip>
		{/if}
		{#if contextWindowUsage}
			<ContextWindowRing
				usedTokens={contextWindowUsage.used}
				limitTokens={contextWindowUsage.limit}
				source={contextWindowUsage.source}
			/>
		{/if}
		{#if streaming}
			<Tooltip label="Stop response" action="stopResponse">
				<button class="stop-button" onclick={() => onStop?.()} aria-label="Stop response">
					<Square size={14} fill="currentColor" stroke-width={0} />
				</button>
			</Tooltip>
		{/if}
		<Tooltip label={sendLabel} action="sendMessage">
			<button
				class="send-button"
				onclick={onSubmit}
				disabled={!canSubmit || disabled || !modelStore.selected}
				aria-label={sendLabel}
			>
				<Send size={16} stroke-width={1.8} />
			</button>
		</Tooltip>
	</div>
</div>

<style>
	.composer-footer {
		position: relative;
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.composer-tools,
	.composer-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.composer-tools {
		flex: 1 1 auto;
	}

	.composer-actions {
		flex: 0 0 auto;
		margin-left: auto;
	}

	.composer-footer .plan-chip {
		display: inline-flex;
		align-items: center;
		padding: 3px 8px;
		border: 1px solid var(--plan-chip-border, var(--plan-border));
		border-radius: 999px;
		background: var(--plan-chip-bg);
		color: var(--plan-chip-text);
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.06em;
		line-height: 1;
		text-transform: uppercase;
		cursor: pointer;
		transition:
			background 140ms ease,
			color 140ms ease;
	}

	.composer-footer .plan-chip:hover:not(:disabled) {
		background: var(--plan-chip-bg-strong, var(--plan-chip-bg));
		color: var(--plan-chip-text-strong, var(--plan-chip-text));
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
		line-height: 1;
	}

	.effort-button.active {
		color: var(--text-muted);
	}

	.effort-button :global(svg) {
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 58%,
			var(--accent, #0066cc)
		);
		transition:
			color 160ms ease,
			filter 160ms ease;
	}

	.effort-button.active :global(svg) {
		color: var(--hero-composer-glow-color, #72c0ff);
		filter: drop-shadow(0 0 5px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.2)));
	}

	.effort-label {
		max-width: 5.5rem;
		overflow: hidden;
		font-size: 11px;
		font-weight: 600;
		line-height: 1;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.send-button {
		display: grid;
		flex-shrink: 0;
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
		flex-shrink: 0;
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
