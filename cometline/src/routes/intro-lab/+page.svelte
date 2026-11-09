<script lang="ts">
	import IntroAnimationV2 from '#lib/features/shell/components/IntroAnimationV2.svelte';

	const frozenMs = $derived.by(() => {
		if (typeof window === 'undefined') return null;
		const raw = new URLSearchParams(window.location.search).get('t');
		if (!raw) return null;
		const n = Number(raw);
		return Number.isFinite(n) ? n : null;
	});

	const showChrome = $derived.by(() => {
		if (typeof window === 'undefined') return true;
		return new URLSearchParams(window.location.search).get('chrome') !== '0';
	});
</script>

<svelte:head>
	<title>Intro lab — Cometline</title>
</svelte:head>

<div class="intro-lab-stage">
	<IntroAnimationV2 debugFrozenElapsed={frozenMs} previewMode={true} />
	<!-- Flight-target stub so preview captures v1-style avatar handoff. -->
	<div class="empty-state lab-avatar-target" aria-hidden="true">
		<div class="avatar"></div>
	</div>
</div>

{#if showChrome}
<p class="hint">
	Dev-only preview route. Visit <code>/intro-lab</code> while running <code>pnpm run dev</code>. Use
	<code>?t=3000</code> to freeze a beat.
</p>
{/if}

<style>
	:global(body) {
		margin: 0;
		overflow: hidden;
	}

	.intro-lab-stage {
		position: fixed;
		inset: 0;
		z-index: 1;
	}

	.intro-lab-stage :global(.intro-v2) {
		z-index: 1;
	}

	.lab-avatar-target {
		position: fixed;
		left: 50%;
		top: calc(50% + 148px);
		transform: translateX(-50%);
		pointer-events: none;
		opacity: 0.001;
	}

	.lab-avatar-target .avatar {
		width: 82px;
		height: 82px;
		border-radius: 50%;
		background: var(--panel-bg, #eef2f6);
	}

	.hint {
		position: fixed;
		top: 8px;
		left: 8px;
		z-index: 2;
		margin: 0;
		padding: 6px 10px;
		font-size: 11px;
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.9);
		color: #333;
		pointer-events: none;
	}
</style>
