<script lang="ts">
	let {
		resizing,
		onPointerDown,
		onPointerMove,
		onPointerUp,
		onKeydown
	}: {
		resizing: boolean;
		onPointerDown: (event: PointerEvent) => void;
		onPointerMove: (event: PointerEvent) => void;
		onPointerUp: (event: PointerEvent) => void;
		onKeydown: (event: KeyboardEvent) => void;
	} = $props();
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="panel-resizer"
	class:resizing
	role="separator"
	aria-orientation="vertical"
	aria-label="Resize workspace panel"
	tabindex="0"
	onpointerdown={onPointerDown}
	onpointermove={onPointerMove}
	onpointerup={onPointerUp}
	onpointercancel={onPointerUp}
	onkeydown={onKeydown}
></div>

<style>
	.panel-resizer {
		flex: 0 0 auto;
		width: 12px;
		margin: 0 -3px 0 -9px;
		z-index: 2;
		cursor: col-resize;
		align-self: stretch;
		background: transparent;
		position: relative;
	}

	.panel-resizer::before {
		content: '';
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		width: 4px;
		height: 36px;
		border-radius: 999px;
		/* background: var(--border-subtle, rgba(148, 163, 184, 0.4)); */
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	.panel-resizer:hover::before,
	.panel-resizer:focus-visible::before,
	.panel-resizer.resizing::before {
		opacity: 1;
	}

	.panel-resizer:focus-visible {
		outline: none;
	}

	@media (max-width: 900px) {
		.panel-resizer {
			display: none;
		}
	}
</style>
