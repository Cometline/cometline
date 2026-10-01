<script lang="ts">
	// Keyframes are global; this component exists so the indicator's scoped CSS stays under budget.
</script>

<style>
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
</style>
