<script lang="ts">
	import { onMount } from 'svelte';
	import type { Session } from '$lib/types';

	let {
		session,
		x,
		y,
		onPin,
		onRename,
		onClose
	}: {
		session: Session;
		x: number;
		y: number;
		onPin: () => void;
		onRename: () => void;
		onClose: () => void;
	} = $props();

	let menuEl = $state<HTMLDivElement | null>(null);

	function handlePin() {
		onPin();
		onClose();
	}

	function handleRename() {
		onRename();
		onClose();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			onClose();
		}
	}

	function handlePointerDown(event: PointerEvent) {
		if (menuEl && !menuEl.contains(event.target as Node)) {
			onClose();
		}
	}

	onMount(() => {
		window.addEventListener('pointerdown', handlePointerDown);
		window.addEventListener('keydown', handleKeydown);
		return () => {
			window.removeEventListener('pointerdown', handlePointerDown);
			window.removeEventListener('keydown', handleKeydown);
		};
	});
</script>

<div
	bind:this={menuEl}
	class="session-context-menu"
	style:left="{x}px"
	style:top="{y}px"
	role="menu"
>
	<button class="menu-item" role="menuitem" onclick={handleRename}>Rename</button>
	<button class="menu-item" role="menuitem" onclick={handlePin}>
		{session.pinned ? 'Unpin session' : 'Pin session'}
	</button>
</div>

<style>
	.session-context-menu {
		position: fixed;
		z-index: 100;
		min-width: 148px;
		padding: 4px;
		border-radius: 8px;
		border: 1px solid var(--border-soft);
		background: var(--surface-elevated, #fff);
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
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
		background: rgba(0, 0, 0, 0.06);
	}
</style>
