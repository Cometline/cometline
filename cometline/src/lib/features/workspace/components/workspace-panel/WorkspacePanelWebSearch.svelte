<script lang="ts">
	import { customCaret } from '$lib/dom/custom-caret';
	import type { WorkspacePanelController } from '$lib/features/workspace/workspace-panel-controller.svelte';
	import type { WorkspacePanelView } from '$lib/features/workspace/workspace-panel-view.svelte';

	let { view, panel }: { view: WorkspacePanelView; panel: WorkspacePanelController } = $props();

	let searchCaretWrap = $state<HTMLDivElement | null>(null);
	let searchCaretEl = $state<HTMLSpanElement | null>(null);
	let searchTrailPoly = $state<SVGPolygonElement | null>(null);
	let searchCaretFocused = $state(false);
	let searchCaretReady = $state(false);
</script>

<div
	class="web-search-stage"
	class:centered={view.showCenteredWebSearch}
	class:overlay={view.showAddressOverlay}
	class:visible={view.showContentSearch}
>
	<div bind:this={searchCaretWrap} class="web-search-box">
		<img class="web-search-icon" src="/app_icon.png" alt="" width="20" height="20" />
		{#if view.searchCaretTrailEnabled}
			<div
				class="web-search-caret-layer"
				class:visible={searchCaretFocused && searchCaretReady}
				aria-hidden="true"
			>
				<svg class="web-search-trail" focusable="false">
					<polygon bind:this={searchTrailPoly}></polygon>
				</svg>
				<span bind:this={searchCaretEl} class="web-search-caret"></span>
			</div>
		{/if}
		<input
			use:customCaret={{
				wrap: searchCaretWrap,
				caret: searchCaretEl,
				trail: searchTrailPoly,
				caretTrail: view.searchCaretTrail,
				color: view.searchCaretColor,
				onStateChange: (state) => {
					searchCaretFocused = state.focused;
					searchCaretReady = state.ready;
				}
			}}
			use:panel.trackAddressInput
			class="address-input web-search-input"
			class:trail-enabled={view.searchCaretTrailEnabled}
			type="text"
			inputmode="search"
			spellcheck="false"
			autocapitalize="off"
			autocomplete="off"
			placeholder="Search Google or type a URL"
			bind:value={() => view.shownAddress, (next) => view.setAddressDraft(next)}
			onfocus={panel.onAddressFocus}
			onblur={panel.onAddressBlur}
			onkeydown={panel.onAddressKeydown}
			aria-label="Address bar"
		/>
	</div>
</div>

<style>
	.address-input {
		flex: 1;
		width: 100%;
		min-width: 0;
		border: none;
		background: transparent;
		font-size: 11px;
		color: var(--text-muted);
		padding: 0;
		outline: none;
	}

	.address-input:focus {
		color: var(--text-main);
	}

	.address-input::placeholder {
		color: var(--text-muted);
		opacity: 0.7;
	}

	.address-input.trail-enabled {
		caret-color: transparent;
	}

	.web-search-stage {
		position: absolute;
		z-index: 4;
		display: flex;
		justify-content: center;
		pointer-events: none;
		visibility: hidden;
	}

	.web-search-stage.visible {
		visibility: visible;
		pointer-events: auto;
	}

	.web-search-stage.centered {
		inset: 0;
		align-items: center;
	}

	.web-search-stage.overlay {
		top: 16px;
		left: 16px;
		right: 16px;
		align-items: flex-start;
	}

	.web-search-box {
		position: relative;
		width: min(32rem, calc(100% - 48px));
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px 16px;
		border-radius: 12px;
		border: 1px solid var(--border-soft);
		background: var(--panel-bg);
		box-shadow: var(--shadow-card);
		box-sizing: border-box;
	}

	.web-search-stage.overlay .web-search-box {
		padding: 8px 12px;
		border-radius: 8px;
	}

	.web-search-input {
		font-size: 14px;
		color: var(--text-main);
	}

	.web-search-icon {
		flex-shrink: 0;
		width: 20px;
		height: 20px;
		border-radius: 5px;
		object-fit: cover;
	}

	.web-search-stage.centered .web-search-icon {
		width: 22px;
		height: 22px;
	}

	.web-search-caret-layer {
		position: absolute;
		inset: 0;
		pointer-events: none;
		z-index: 2;
		overflow: hidden;
		opacity: 0;
		transition: opacity 0.08s ease;
	}

	.web-search-caret-layer.visible {
		opacity: 1;
	}

	.web-search-trail {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		overflow: visible;
	}

	.web-search-trail polygon {
		fill: var(--rce-caret-color);
		stroke: none;
		opacity: 0;
		filter: drop-shadow(0 0 6px var(--rce-caret-color));
	}

	@keyframes web-search-caret-blink {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.75;
		}
	}

	.web-search-caret {
		position: absolute;
		top: 0;
		left: 0;
		width: 2px;
		height: 1.2em;
		border-radius: 999px;
		background: var(--rce-caret-color);
		box-shadow: 0 0 9px var(--rce-caret-color);
		will-change: transform;
		animation: web-search-caret-blink 1.1s ease-in-out infinite;
	}

	:global(.web-search-caret.moving) {
		animation: none;
	}

	.web-search-caret::after {
		content: '';
		position: absolute;
		inset: -5px -4px;
		border-radius: 999px;
		background: var(--rce-caret-color);
		opacity: 0.14;
		filter: blur(5px);
	}
</style>
