<script lang="ts">
	import { onMount } from 'svelte';
	import { portal } from '$lib/components/portal';

	let {
		position,
		onAdd,
		onDismiss
	}: {
		position: { top: number; left: number };
		onAdd: () => void;
		onDismiss: () => void;
	} = $props();

	let button = $state<HTMLButtonElement | null>(null);

	function handlePointerDown(event: PointerEvent) {
		if (button?.contains(event.target as Node)) return;
		onDismiss();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') onDismiss();
	}

	onMount(() => {
		window.addEventListener('pointerdown', handlePointerDown, true);
		window.addEventListener('keydown', handleKeydown);
		document.addEventListener('scroll', onDismiss, true);
		window.addEventListener('resize', onDismiss);
		return () => {
			window.removeEventListener('pointerdown', handlePointerDown, true);
			window.removeEventListener('keydown', handleKeydown);
			document.removeEventListener('scroll', onDismiss, true);
			window.removeEventListener('resize', onDismiss);
		};
	});
</script>

<button
	bind:this={button}
	use:portal
	type="button"
	class="selection-add-chat"
	style:top="{position.top}px"
	style:left="{position.left}px"
	onmousedown={(event) => event.preventDefault()}
	onclick={onAdd}
>
	Add to chat
</button>

<style>
	.selection-add-chat {
		position: fixed;
		z-index: 10000;
		padding: 6px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: #fff;
		color: var(--text-main);
		font-size: 12px;
		font-weight: 600;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
		cursor: pointer;
	}

	.selection-add-chat:hover {
		border-color: var(--text-soft);
	}
</style>
