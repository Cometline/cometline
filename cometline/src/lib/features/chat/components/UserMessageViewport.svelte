<script lang="ts">
	import type { Snippet } from 'svelte';
	import { ChevronDown } from '@lucide/svelte';

	let {
		contentId,
		interactive = true,
		children
	}: {
		contentId?: string;
		interactive?: boolean;
		children: Snippet;
	} = $props();

	let viewport = $state<HTMLDivElement | null>(null);
	let content = $state<HTMLDivElement | null>(null);
	let expanded = $state(false);
	let overflowing = $state(false);
	let contentHeight = $state(0);

	function measure() {
		if (!viewport || !content) return;
		contentHeight = content.scrollHeight;
		if (!expanded) {
			overflowing = content.scrollHeight - viewport.clientHeight > 1;
		}
	}

	$effect(() => {
		if (!viewport || !content) return;
		measure();
		if (typeof ResizeObserver === 'undefined') return;

		const observer = new ResizeObserver(measure);
		observer.observe(viewport);
		observer.observe(content);
		return () => observer.disconnect();
	});

	function toggleExpanded() {
		if (!interactive || !overflowing) return;
		measure();
		expanded = !expanded;
	}
</script>

<div
	bind:this={viewport}
	id={contentId}
	data-user-message-viewport
	class="user-message-viewport"
	class:expanded
	class:overflowing
	style:max-height={expanded && contentHeight > 0 ? `${contentHeight}px` : undefined}
>
	<div bind:this={content} class="user-message-content" class:reserve-control={overflowing}>
		{@render children()}
	</div>
</div>

{#if overflowing}
	{#if interactive}
		<button
			type="button"
			data-user-message-expand
			class="user-message-expand"
			class:expanded
			aria-label={expanded ? 'Collapse user message' : 'Expand user message'}
			aria-expanded={expanded}
			aria-controls={contentId}
			onclick={toggleExpanded}
		>
			<ChevronDown size={15} stroke-width={2} />
		</button>
	{:else}
		<span class="user-message-expand" aria-hidden="true">
			<ChevronDown size={15} stroke-width={2} />
		</span>
	{/if}
{/if}

<style>
	.user-message-viewport {
		max-height: var(--user-message-collapsed-height, min(18rem, 42dvh));
		overflow: clip;
		transition: max-height var(--duration-fast) var(--ease-smooth);
	}

	.user-message-viewport.overflowing:not(.expanded) {
		-webkit-mask-image: linear-gradient(to bottom, #000 0%, #000 76%, transparent 100%);
		mask-image: linear-gradient(to bottom, #000 0%, #000 76%, transparent 100%);
	}

	.user-message-content {
		min-width: 0;
	}

	.user-message-content.reserve-control {
		padding-bottom: 30px;
	}

	.user-message-expand {
		position: absolute;
		right: 8px;
		bottom: 7px;
		z-index: 2;
		display: grid;
		place-items: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
		border-radius: 999px;
		background: color-mix(in srgb, var(--panel-bg) 90%, transparent);
		box-shadow: 0 3px 10px var(--user-bubble-shadow);
		color: var(--text-muted);
	}

	button.user-message-expand {
		cursor: pointer;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			color var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	button.user-message-expand:hover,
	button.user-message-expand:focus-visible {
		border-color: color-mix(in srgb, var(--hero-composer-glow-color) 48%, var(--border-soft));
		background: var(--panel-bg);
		color: var(--text-main);
		outline: none;
	}

	.user-message-expand :global(svg) {
		transition: transform var(--duration-fast) var(--ease-smooth);
	}

	.user-message-expand.expanded :global(svg) {
		transform: rotate(180deg);
	}

	@media (prefers-reduced-motion: reduce) {
		.user-message-viewport,
		button.user-message-expand,
		.user-message-expand :global(svg) {
			transition: none;
		}
	}
</style>
