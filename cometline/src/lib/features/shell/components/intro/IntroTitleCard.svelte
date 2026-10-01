<script lang="ts">
	let {
		avatarSrc,
		avatarSrcset,
		showWordmark,
		showTagline,
		showFlyIcon,
		projectIconRef = $bindable(null)
	}: {
		avatarSrc: string;
		avatarSrcset: string | undefined;
		showWordmark: boolean;
		showTagline: boolean;
		showFlyIcon: boolean;
		projectIconRef?: HTMLImageElement | null;
	} = $props();
</script>

<div class="card">
	<img
		bind:this={projectIconRef}
		class="project-icon"
		class:in={showWordmark}
		class:is-flying={showFlyIcon}
		src={avatarSrc}
		srcset={avatarSrcset}
		sizes="82px"
		alt=""
	/>
	<h1 class="wordmark" class:in={showWordmark}>
		<span class="lead">Comet</span><span class="trail">line</span>
	</h1>
	<p class="tagline" class:in={showTagline}>A thought, a task, a file — Cometline continues.</p>
</div>

<style>
	.card {
		position: relative;
		z-index: 2;
		text-align: center;
		transform: translateY(58px);
		pointer-events: none;
		user-select: none;
	}

	.project-icon {
		display: block;
		width: 120px;
		height: 120px;
		margin: 0 auto 22px;
		border-radius: 50%;
		box-shadow: var(--shadow-card);
		opacity: 0;
		transform: scale(0.92);
		transition:
			opacity 0.9s var(--ease-intro, ease),
			transform 0.9s var(--ease-intro, ease);
	}

	.project-icon.in {
		opacity: 1;
		transform: scale(1);
	}

	.project-icon.is-flying {
		opacity: 0;
		transition: none;
	}

	.wordmark {
		margin: 0;
		font-family:
			'Hoefler Text', 'Iowan Old Style', 'Palatino Linotype', Palatino, Georgia, serif;
		font-weight: 600;
		letter-spacing: 0.12em;
		font-size: clamp(38px, 6.4vw, 76px);
		color: var(--intro-ink, var(--intro-ink));
		opacity: 0;
		filter: blur(8px);
		transform: translateY(10px);
		transition:
			opacity 0.9s var(--ease-intro, ease),
			filter 0.9s var(--ease-intro, ease),
			transform 0.9s var(--ease-intro, ease);
		text-shadow: 0 1px 0 rgba(255, 255, 255, 0.6);
	}

	.wordmark.in {
		opacity: 1;
		filter: blur(0);
		transform: translateY(0);
	}

	.wordmark .lead {
		color: var(--intro-ink, var(--intro-ink));
	}

	.wordmark .trail {
		color: var(--hero-composer-glow-color, var(--color-72c0ff));
	}

	.tagline {
		margin: 18px 0 0;
		font-family: 'Hoefler Text', 'Iowan Old Style', Georgia, serif;
		font-style: italic;
		font-size: clamp(13px, 1.7vw, 17px);
		letter-spacing: 0.04em;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--color-72c0ff)) 42%,
			var(--intro-ink, var(--intro-ink))
		);
		opacity: 0;
		transform: translateY(8px);
		transition:
			opacity 1s var(--ease-intro, ease),
			transform 1s var(--ease-intro, ease);
	}

	.tagline.in {
		opacity: 1;
		transform: translateY(0);
	}

	:global(.is-exit) .card {
		transition:
			transform 0.76s var(--ease-intro, ease),
			opacity 0.6s var(--ease-intro, ease);
		transform: translateY(58px) scale(1.04);
		opacity: 0;
	}

	@media (prefers-reduced-motion: reduce) {
		.wordmark,
		.tagline {
			animation: none !important;
			transition: opacity 0.3s linear !important;
		}
		.wordmark,
		.tagline {
			filter: none;
			transform: none;
		}
	}
</style>
