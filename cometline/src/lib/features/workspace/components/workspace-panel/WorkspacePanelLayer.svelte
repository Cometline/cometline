<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		active,
		variant,
		children
	}: {
		active: boolean;
		variant?: 'content' | 'terminal';
		children: Snippet;
	} = $props();
</script>

<div
	class="panel-layer"
	class:panel-layer-content={variant === 'content'}
	class:panel-layer-terminal={variant === 'terminal'}
	class:active
	inert={!active}
	aria-hidden={!active}
>
	{@render children()}
</div>

<style>
	.panel-layer {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: var(--panel-bg);
		pointer-events: none;
		visibility: hidden;
		z-index: 1;
	}

	.panel-layer.active {
		pointer-events: auto;
		visibility: visible;
		z-index: 2;
	}

	.panel-layer-content.active,
	.panel-layer-terminal.active {
		z-index: 3;
	}

	.panel-layer :global(.file-tree-browser),
	.panel-layer :global(.git-changes),
	.panel-layer :global(.git-diff-view) {
		flex: 1;
		min-height: 0;
		height: 100%;
	}
</style>
