<script lang="ts">
	import { onMount } from 'svelte';
	import type { GitGraphMenuItem } from '#lib/features/workspace/git-graph.js';

	let {
		x,
		y,
		items,
		onChoose,
		onClose
	}: {
		x: number;
		y: number;
		items: GitGraphMenuItem[];
		onChoose: (id: GitGraphMenuItem['id']) => void;
		onClose: () => void;
	} = $props();

	let menuEl = $state<HTMLDivElement | null>(null);

	onMount(() => {
		function onKey(event: KeyboardEvent) {
			if (event.key === 'Escape') onClose();
		}
		function onPointer(event: PointerEvent) {
			if (menuEl && !menuEl.contains(event.target as Node)) onClose();
		}
		window.addEventListener('keydown', onKey);
		window.addEventListener('pointerdown', onPointer);
		return () => {
			window.removeEventListener('keydown', onKey);
			window.removeEventListener('pointerdown', onPointer);
		};
	});
</script>

<div bind:this={menuEl} class="git-graph-menu" style:left="{x}px" style:top="{y}px" role="menu">
	{#each items as item (item.id)}
		<button
			type="button"
			class="menu-item"
			class:danger={item.id === 'delete'}
			role="menuitem"
			onclick={() => onChoose(item.id)}
		>
			{item.label}
		</button>
	{/each}
</div>

<style>
	.git-graph-menu {
		position: fixed;
		z-index: 100;
		min-width: 168px;
		padding: 4px;
		border-radius: 8px;
		border: 1px solid var(--border-soft);
		background: var(--surface-elevated, var(--panel-bg));
		box-shadow: var(--shadow-card);
	}

	.menu-item {
		display: block;
		width: 100%;
		padding: 7px 10px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-main);
		font-size: 13px;
		line-height: 1.35;
		text-align: left;
		cursor: pointer;
	}

	.menu-item:hover {
		background: color-mix(in srgb, var(--text-main) 8%, transparent);
	}

	.menu-item.danger {
		color: var(--status-error);
	}
</style>
