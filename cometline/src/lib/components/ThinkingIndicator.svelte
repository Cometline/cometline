<script lang="ts">
	interface Props {
		/** Hex color for the comets and core. Falls back to the hero glow color. */
		color?: string;
		/** Rendered height/width in px. The indicator is square. Defaults to 24. */
		size?: number;
		/** Accessible label for the thinking state. */
		label?: string;
	}

	let { color, size = 24, label = 'Assistant is thinking' }: Props = $props();
</script>

<div
	class="thinking-indicator"
	role="status"
	aria-label={label}
	style:--thinking-color={color}
	style:--thinking-scale={size / 24}
>
	<div class="thinking-stage" aria-hidden="true">
		<div class="thinking-core"></div>
		<div class="thinking-comet thinking-comet--a">
			<div class="thinking-tail"></div>
			<div class="thinking-head"></div>
		</div>
		<div class="thinking-comet thinking-comet--b">
			<div class="thinking-tail"></div>
			<div class="thinking-head"></div>
		</div>
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
	.thinking-comet {
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
			transform: translate(0px, 5px) rotate(180deg) scale(1);
		}
		87.5% {
			transform: translate(5.66px, 3.54px) rotate(148deg) scale(0.975);
		}
		100% {
			transform: translate(8px, 0px) rotate(90deg) scale(0.95);
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

	@media (prefers-reduced-motion: reduce) {
		.thinking-comet {
			animation: none;
			opacity: 0;
		}

		.thinking-core {
			animation: thinking-core-pulse 2.4s ease-in-out infinite;
		}
	}
</style>
