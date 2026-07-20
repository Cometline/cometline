<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onDestroy, tick } from 'svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import type { ShortcutAction } from '$lib/keyboard-shortcuts';
	import { shortcutTooltipKbd } from './shortcut-tooltip';
	import { clampTooltipPosition } from './tooltip-position';
	import { portal } from './portal';

	let {
		label,
		action,
		disabled = false,
		delay = 350,
		children
	}: {
		label: string;
		action?: ShortcutAction;
		disabled?: boolean;
		delay?: number;
		children: Snippet;
	} = $props();

	const tipId = `tooltip-${Math.random().toString(36).slice(2, 10)}`;
	let wrapEl = $state<HTMLElement | null>(null);
	let bubbleEl = $state<HTMLElement | null>(null);
	let open = $state(false);
	let placed = $state(false);
	let tipTop = $state(0);
	let tipLeft = $state(0);
	let showTimer: ReturnType<typeof setTimeout> | null = null;
	let describedEl: HTMLElement | null = null;

	const kbd = $derived(shortcutTooltipKbd(action, settingsStore.settings.shortcuts));

	function clearTimer() {
		if (showTimer) {
			clearTimeout(showTimer);
			showTimer = null;
		}
	}

	function findTarget(): HTMLElement | null {
		if (!wrapEl) return null;
		return (
			wrapEl.querySelector<HTMLElement>('button, a, [role="button"], input, select, textarea') ??
			(wrapEl.firstElementChild as HTMLElement | null)
		);
	}

	function setDescribedBy(on: boolean) {
		const target = findTarget();
		if (describedEl && describedEl !== target) {
			describedEl.removeAttribute('aria-describedby');
			describedEl = null;
		}
		if (!target) return;
		if (on) {
			target.setAttribute('aria-describedby', tipId);
			describedEl = target;
		} else {
			target.removeAttribute('aria-describedby');
			describedEl = null;
		}
	}

	function effectiveDelay() {
		if (
			typeof window !== 'undefined' &&
			window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
		) {
			return 0;
		}
		return delay;
	}

	function reposition() {
		if (!wrapEl || !bubbleEl) return;
		const anchor = wrapEl.getBoundingClientRect();
		const tip = bubbleEl.getBoundingClientRect();
		const pos = clampTooltipPosition({
			anchor,
			tip: { width: tip.width, height: tip.height },
			viewport: { width: window.innerWidth, height: window.innerHeight }
		});
		tipTop = pos.top;
		tipLeft = pos.left;
	}

	async function show() {
		if (disabled) return;
		clearTimer();
		showTimer = setTimeout(async () => {
			open = true;
			placed = false;
			setDescribedBy(true);
			await tick();
			reposition();
			placed = true;
		}, effectiveDelay());
	}

	function hide() {
		clearTimer();
		open = false;
		placed = false;
		setDescribedBy(false);
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) {
			hide();
		}
	}

	function onViewportChange() {
		if (open) reposition();
	}

	onDestroy(() => {
		clearTimer();
		setDescribedBy(false);
	});
</script>

<svelte:window onresize={onViewportChange} onscroll={onViewportChange} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<span
	class="tooltip-wrap"
	bind:this={wrapEl}
	onmouseenter={show}
	onmouseleave={hide}
	onfocusin={show}
	onfocusout={hide}
	onkeydown={onKeydown}
>
	{@render children()}
	{#if open}
		<span
			class="tooltip-bubble"
			class:placed
			id={tipId}
			role="tooltip"
			use:portal
			bind:this={bubbleEl}
			style:top="{tipTop}px"
			style:left="{tipLeft}px"
		>
			<span class="tooltip-label">{label}</span>
			{#if kbd}
				<kbd>{kbd}</kbd>
			{/if}
		</span>
	{/if}
</span>

<style>
	.tooltip-wrap {
		position: relative;
		display: inline-flex;
		align-items: center;
		vertical-align: middle;
		max-width: 100%;
	}

	.tooltip-bubble {
		position: fixed;
		z-index: 10000;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 8px;
		border-radius: 8px;
		border: 1px solid var(--border-soft);
		background: var(--panel-bg, rgba(255, 255, 255, 0.96));
		color: var(--text-main);
		font-size: 12px;
		font-weight: 550;
		font-style: normal;
		letter-spacing: normal;
		line-height: 1.2;
		text-transform: none;
		white-space: nowrap;
		pointer-events: none;
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.1);
		opacity: 0;
		visibility: hidden;
	}

	.tooltip-bubble.placed {
		opacity: 1;
		visibility: visible;
		animation: tooltip-fade 120ms ease-out;
	}

	.tooltip-label {
		color: var(--text-main);
	}

	kbd {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 1px 5px;
		border: 1px solid var(--border-soft);
		border-radius: 5px;
		background: color-mix(in srgb, var(--panel-bg, #fff) 70%, var(--text-soft, #999) 12%);
		font-family: inherit;
		font-size: 11px;
		font-weight: 650;
		color: var(--text-muted, var(--text-soft));
		line-height: 1.3;
	}

	@keyframes tooltip-fade {
		from {
			opacity: 0;
			transform: translateY(2px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.tooltip-bubble.placed {
			animation: none;
		}
	}
</style>
