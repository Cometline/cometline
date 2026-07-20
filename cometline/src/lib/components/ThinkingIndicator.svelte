<script lang="ts">
	import { type ThinkingIndicatorVariant } from './thinking-indicator';

	interface Props {
		/** Hex color for the comets and core. Falls back to the hero glow color. */
		color?: string;
		/** Rendered height/width in px. The indicator is square. Defaults to 24. */
		size?: number;
		/** Accessible label for the thinking state. */
		label?: string;
		/**
		 * Celestial motion style.
		 * - orbit: dual comets around a pulsing core (default / thinking + prep)
		 * - eclipse: bright limb with a dark core drift (running tools)
		 * - nova: pulsing core with brief radial spark flashes (composing)
		 *
		 * Live turns usually pass a sticky variant from turn-status phase
		 * via `AssistantThinkingWait` (see `stageForTurnPhase`).
		 */
		variant?: ThinkingIndicatorVariant;
	}

	let {
		color,
		size = 24,
		label = 'Assistant is thinking',
		variant = 'orbit'
	}: Props = $props();
</script>

<div
	class="thinking-indicator"
	class:variant-orbit={variant === 'orbit'}
	class:variant-eclipse={variant === 'eclipse'}
	class:variant-nova={variant === 'nova'}
	role="status"
	aria-label={label}
	style:--thinking-color={color}
	style:--thinking-scale={size / 24}
>
	<div class="thinking-stage" aria-hidden="true">
		{#if variant === 'orbit'}
			<div class="thinking-core"></div>
			<div class="thinking-comet thinking-comet--a">
				<div class="thinking-tail"></div>
				<div class="thinking-head"></div>
			</div>
			<div class="thinking-comet thinking-comet--b">
				<div class="thinking-tail"></div>
				<div class="thinking-head"></div>
			</div>
		{:else if variant === 'eclipse'}
			<div class="thinking-eclipse-glow"></div>
			<div class="thinking-eclipse-disk"></div>
			<div class="thinking-eclipse-limb"></div>
		{:else}
			<div class="thinking-nova-rays"></div>
			<div class="thinking-core thinking-core--nova"></div>
			<div class="thinking-spark thinking-spark--a"></div>
			<div class="thinking-spark thinking-spark--b"></div>
			<div class="thinking-spark thinking-spark--c"></div>
		{/if}
	</div>
</div>

<style>
	.thinking-indicator {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: calc(24px * var(--thinking-scale, 1));
		height: calc(24px * var(--thinking-scale, 1));
		color: var(--thinking-color, var(--hero-composer-glow-color, #72c0ff));
	}

	.thinking-stage {
		position: absolute;
		width: 24px;
		height: 24px;
		transform: scale(var(--thinking-scale, 1));
		transform-origin: top left;
	}

	.thinking-core,
	.thinking-comet,
	.thinking-nova-rays,
	.thinking-spark,
	.thinking-eclipse-glow,
	.thinking-eclipse-disk,
	.thinking-eclipse-limb {
		position: absolute;
		top: 50%;
		left: 50%;
	}

	.thinking-core {
		width: 3px;
		height: 3px;
		border-radius: 999px;
		background: currentColor;
		transform: translate(-50%, -50%);
		opacity: 0.3;
		transform-origin: center;
		animation: thinking-core-pulse 1.6s cubic-bezier(0.45, 0, 0.2, 1) infinite;
		will-change: transform, opacity;
	}

	.thinking-core--nova {
		width: 4px;
		height: 4px;
		animation: thinking-nova-core 1.8s cubic-bezier(0.45, 0, 0.2, 1) infinite;
	}

	.thinking-comet {
		width: 0;
		height: 0;
		transform-origin: 0 0;
		animation: thinking-orbit 1.5s linear infinite;
		will-change: transform;
	}

	.thinking-comet--b {
		animation-delay: -0.75s;
	}

	.thinking-head {
		position: absolute;
		width: 4px;
		height: 4px;
		border-radius: 999px;
		background: currentColor;
		box-shadow: 0 0 6px 1px currentColor;
		transform: translate(-50%, -50%);
	}

	.thinking-tail {
		position: absolute;
		left: 0;
		top: 50%;
		width: 7px;
		height: 2.5px;
		transform: translateY(-50%);
		background: linear-gradient(to right, currentColor 30%, transparent 100%);
		border-radius: 999px;
		filter: blur(0.4px);
		opacity: 0.65;
	}

	.thinking-nova-rays {
		width: 16px;
		height: 16px;
		transform: translate(-50%, -50%);
		background:
			linear-gradient(currentColor, currentColor) center / 1px 100% no-repeat,
			linear-gradient(currentColor, currentColor) center / 100% 1px no-repeat;
		opacity: 0;
		animation: thinking-nova-rays 1.8s ease-in-out infinite;
		filter: blur(0.2px);
	}

	.thinking-spark {
		width: 2px;
		height: 2px;
		border-radius: 999px;
		background: currentColor;
		box-shadow: 0 0 4px currentColor;
		opacity: 0;
		will-change: transform, opacity;
	}

	.thinking-spark--a {
		animation: thinking-spark-a 1.8s ease-out infinite;
	}

	.thinking-spark--b {
		animation: thinking-spark-b 1.8s ease-out infinite;
		animation-delay: 0.15s;
	}

	.thinking-spark--c {
		animation: thinking-spark-c 1.8s ease-out infinite;
		animation-delay: 0.3s;
	}

	.thinking-eclipse-glow {
		width: 14px;
		height: 14px;
		border-radius: 999px;
		transform: translate(-50%, -50%);
		background: radial-gradient(
			circle,
			color-mix(in srgb, currentColor 55%, transparent) 0%,
			transparent 70%
		);
		animation: thinking-eclipse-glow 2.2s ease-in-out infinite;
	}

	.thinking-eclipse-disk {
		width: 8px;
		height: 8px;
		border-radius: 999px;
		transform: translate(-50%, -50%);
		background: color-mix(in srgb, var(--panel-bg, #0b1020) 82%, currentColor 8%);
		box-shadow: inset 0 0 0 1px color-mix(in srgb, currentColor 20%, transparent);
		animation: thinking-eclipse-drift 2.2s ease-in-out infinite;
	}

	.thinking-eclipse-limb {
		width: 9px;
		height: 9px;
		border-radius: 999px;
		transform: translate(-50%, -50%);
		box-shadow: 1.5px 0 4px 0 currentColor;
		animation: thinking-eclipse-limb 2.2s ease-in-out infinite;
	}

	@keyframes thinking-orbit {
		0% {
			transform: translate(8px, 0px) rotate(90deg) scale(0.95);
		}
		12.5% {
			transform: translate(5.66px, -3.54px) rotate(32deg) scale(0.975);
		}
		25% {
			transform: translate(0px, -5px) rotate(0deg) scale(1);
		}
		37.5% {
			transform: translate(-5.66px, -3.54px) rotate(-32deg) scale(1.025);
		}
		50% {
			transform: translate(-8px, 0px) rotate(-90deg) scale(1.05);
		}
		62.5% {
			transform: translate(-5.66px, 3.54px) rotate(-148deg) scale(1.025);
		}
		75% {
			transform: translate(0px, 5px) rotate(-180deg) scale(1);
		}
		87.5% {
			transform: translate(5.66px, 3.54px) rotate(-212deg) scale(0.975);
		}
		100% {
			transform: translate(8px, 0px) rotate(-270deg) scale(0.95);
		}
	}

	@keyframes thinking-core-pulse {
		0%,
		100% {
			opacity: 0.3;
			transform: translate(-50%, -50%) scale(0.9);
		}
		50% {
			opacity: 0.7;
			transform: translate(-50%, -50%) scale(1.2);
		}
	}

	@keyframes thinking-nova-core {
		0%,
		100% {
			opacity: 0.35;
			transform: translate(-50%, -50%) scale(0.85);
		}
		40% {
			opacity: 1;
			transform: translate(-50%, -50%) scale(1.35);
		}
		55% {
			opacity: 0.7;
			transform: translate(-50%, -50%) scale(1.1);
		}
	}

	@keyframes thinking-nova-rays {
		0%,
		35%,
		100% {
			opacity: 0;
			transform: translate(-50%, -50%) scale(0.4) rotate(0deg);
		}
		45% {
			opacity: 0.55;
			transform: translate(-50%, -50%) scale(1) rotate(18deg);
		}
		60% {
			opacity: 0;
			transform: translate(-50%, -50%) scale(1.25) rotate(28deg);
		}
	}

	@keyframes thinking-spark-a {
		0%,
		40% {
			opacity: 0;
			transform: translate(-50%, -50%) translate(0, 0) scale(0.5);
		}
		48% {
			opacity: 1;
		}
		70%,
		100% {
			opacity: 0;
			transform: translate(-50%, -50%) translate(7px, -6px) scale(0.3);
		}
	}

	@keyframes thinking-spark-b {
		0%,
		40% {
			opacity: 0;
			transform: translate(-50%, -50%) translate(0, 0) scale(0.5);
		}
		48% {
			opacity: 1;
		}
		70%,
		100% {
			opacity: 0;
			transform: translate(-50%, -50%) translate(-7px, -5px) scale(0.3);
		}
	}

	@keyframes thinking-spark-c {
		0%,
		40% {
			opacity: 0;
			transform: translate(-50%, -50%) translate(0, 0) scale(0.5);
		}
		48% {
			opacity: 1;
		}
		70%,
		100% {
			opacity: 0;
			transform: translate(-50%, -50%) translate(1px, 8px) scale(0.3);
		}
	}

	@keyframes thinking-eclipse-glow {
		0%,
		100% {
			opacity: 0.35;
			transform: translate(-50%, -50%) scale(0.92);
		}
		50% {
			opacity: 0.75;
			transform: translate(-50%, -50%) scale(1.08);
		}
	}

	@keyframes thinking-eclipse-drift {
		0%,
		100% {
			transform: translate(calc(-50% + 0.6px), -50%);
		}
		50% {
			transform: translate(calc(-50% - 0.8px), -50%);
		}
	}

	@keyframes thinking-eclipse-limb {
		0%,
		100% {
			opacity: 0.55;
			transform: translate(-50%, -50%) rotate(-12deg);
		}
		50% {
			opacity: 1;
			transform: translate(-50%, -50%) rotate(18deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.thinking-comet,
		.thinking-nova-rays,
		.thinking-spark,
		.thinking-eclipse-glow,
		.thinking-eclipse-disk,
		.thinking-eclipse-limb {
			animation: none;
		}

		.thinking-comet,
		.thinking-spark,
		.thinking-nova-rays {
			opacity: 0;
		}

		.thinking-eclipse-glow,
		.thinking-eclipse-limb {
			opacity: 0.55;
		}

		.thinking-core {
			animation: thinking-core-pulse 2.4s ease-in-out infinite;
		}
	}
</style>
