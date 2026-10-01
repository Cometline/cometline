<script lang="ts">
	import { Send, Square } from '@lucide/svelte';
	import ContextWindowRing from '$lib/features/composer/components/ContextWindowRing.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import type { AgentMode } from '$lib/types';

	let {
		contextWindowUsage,
		streaming,
		canSubmit,
		disabled,
		agentMode,
		agentModeKnown,
		onSwitchToAuto,
		onStop,
		onSubmit
	}: {
		contextWindowUsage: { used: number; limit: number; source: 'server' | 'fallback' } | null;
		streaming: boolean;
		canSubmit: boolean;
		disabled: boolean;
		agentMode: AgentMode;
		agentModeKnown: boolean;
		onSwitchToAuto: () => void | Promise<void>;
		onStop?: () => void;
		onSubmit: () => void;
	} = $props();

	const sendLabel = $derived(streaming ? 'Queue follow-up' : 'Send');
</script>

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

<style>
	.composer-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		flex: 0 0 auto;
		margin-left: auto;
	}

	.composer-actions button {
		border: none;
		background: transparent;
		color: var(--text-muted);
		border-radius: 7px;
		font-size: 13px;
		cursor: pointer;
	}

	.composer-actions button:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.composer-actions button:active:not(:disabled) {
		background: rgba(0, 0, 0, 0.07);
	}

	.composer-actions button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.plan-chip {
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

	.plan-chip:hover:not(:disabled) {
		background: var(--plan-chip-bg-strong, var(--plan-chip-bg));
		color: var(--plan-chip-text-strong, var(--plan-chip-text));
	}

	.send-button {
		display: grid;
		flex-shrink: 0;
		place-items: center;
		padding: 6px;
		border-radius: 999px;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--color-72c0ff)) 58%,
			var(--accent, var(--accent))
		) !important;
		transition:
			color 160ms ease,
			background 160ms ease,
			box-shadow 160ms ease;
	}

	.send-button:hover:not(:disabled) {
		color: var(--hero-composer-glow-color, var(--color-72c0ff)) !important;
		background: var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24)) !important;
		box-shadow: 0 0 14px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.send-button:active:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--color-72c0ff)) 22%,
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
			var(--hero-composer-glow-color, var(--color-72c0ff)) 58%,
			var(--accent, var(--accent))
		) !important;
		transition:
			color 160ms ease,
			background 160ms ease,
			box-shadow 160ms ease;
	}

	.stop-button:hover:not(:disabled) {
		color: var(--hero-composer-glow-color, var(--color-72c0ff)) !important;
		background: var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24)) !important;
		box-shadow: 0 0 14px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.stop-button:active:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--color-72c0ff)) 22%,
			transparent
		) !important;
		box-shadow: 0 0 8px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}
</style>
