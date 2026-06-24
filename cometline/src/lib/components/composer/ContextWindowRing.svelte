<script lang="ts">
	import { formatContextPercent, formatContextUsageTokens } from '$lib/context-window';

	let { usedTokens, limitTokens }: { usedTokens: number; limitTokens: number } = $props();

	const radius = 8;
	const circumference = 2 * Math.PI * radius;

	let percent = $derived(limitTokens > 0 ? Math.min(100, (usedTokens / limitTokens) * 100) : 0);
	let displayPercent = $derived(usedTokens > 0 && percent < 1.5 ? 1.5 : percent);
	let dashOffset = $derived(circumference - (displayPercent / 100) * circumference);
	let level = $derived(percent >= 90 ? 'critical' : percent >= 75 ? 'high' : 'normal');
	let tooltipLine = $derived(
		`${formatContextPercent(usedTokens, limitTokens)}% · ${formatContextUsageTokens(usedTokens)} / ${formatContextUsageTokens(limitTokens)} context used`
	);
</script>

<div class="context-ring-wrap">
	<button
		type="button"
		class="context-ring-trigger"
		class:level-high={level === 'high'}
		class:level-critical={level === 'critical'}
		aria-label={tooltipLine}
	>
		<svg viewBox="0 0 20 20" width="20" height="20" aria-hidden="true">
			<circle class="track" cx="10" cy="10" r={radius} fill="none" stroke-width="2.25" />
			<circle
				class="progress"
				cx="10"
				cy="10"
				r={radius}
				fill="none"
				stroke-width="2.25"
				stroke-dasharray={circumference}
				stroke-dashoffset={dashOffset}
			/>
		</svg>
	</button>
	<div class="context-tooltip" role="tooltip">
		<p class="context-tooltip-main">{tooltipLine}</p>
		<p class="context-tooltip-note">Estimated from visible transcript</p>
	</div>
</div>

<style>
	.context-ring-wrap {
		position: relative;
		display: flex;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
	}

	.context-ring-trigger {
		all: unset;
		box-sizing: border-box;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border-radius: 50%;
		cursor: default;
	}

	.context-ring-trigger svg {
		display: block;
		overflow: visible;
		transform: rotate(-90deg);
	}

	/* Light blue empty ring — follows hero glow palette */
	.track {
		stroke: color-mix(in srgb, var(--hero-composer-glow-color, #72c0ff) 42%, white);
	}

	/* Filled arc deepens toward accent as context grows */
	.progress {
		stroke: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 58%,
			var(--accent, #0066cc)
		);
		stroke-linecap: round;
		transition: stroke-dashoffset 180ms ease;
	}

	.context-ring-trigger.level-high .progress {
		stroke: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 35%,
			var(--accent, #0066cc)
		);
	}

	.context-ring-trigger.level-critical .progress {
		stroke: var(--accent, #0066cc);
	}

	.context-tooltip {
		position: absolute;
		right: -4px;
		bottom: calc(100% + 10px);
		min-width: 220px;
		padding: 8px 10px;
		border-radius: 8px;
		border: 1px solid var(--border-soft);
		background: rgba(28, 28, 30, 0.96);
		color: #f5f5f7;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.28);
		opacity: 0;
		pointer-events: none;
		transform: translateY(4px);
		transition:
			opacity 140ms ease,
			transform 140ms ease;
		z-index: 20;
	}

	.context-tooltip::after {
		content: '';
		position: absolute;
		right: 12px;
		bottom: -5px;
		width: 8px;
		height: 8px;
		background: rgba(28, 28, 30, 0.96);
		border-right: 1px solid var(--border-soft);
		border-bottom: 1px solid var(--border-soft);
		transform: rotate(45deg);
	}

	.context-ring-wrap:hover .context-tooltip,
	.context-ring-wrap:focus-within .context-tooltip {
		opacity: 1;
		transform: translateY(0);
	}

	.context-tooltip-main {
		margin: 0;
		font-size: 12px;
		font-variant-numeric: tabular-nums;
		line-height: 1.35;
	}

	.context-tooltip-note {
		margin: 4px 0 0;
		font-size: 11px;
		line-height: 1.35;
		color: rgba(245, 245, 247, 0.62);
	}
</style>
