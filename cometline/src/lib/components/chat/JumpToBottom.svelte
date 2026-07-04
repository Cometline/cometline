<script lang="ts">
	import { ChevronDown } from '@lucide/svelte';

	let { onclick }: { onclick: () => void } = $props();
</script>

<button type="button" class="jump-to-bottom" {onclick} aria-label="Jump to latest">
	<ChevronDown size={16} />
	<span>Jump to latest</span>
</button>

<style>
	.jump-to-bottom {
		position: absolute;
		bottom: 20px;
		left: 50%;
		transform: translateX(-50%);
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 8px 14px;
		border: 1px solid var(--border-soft);
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-main);
		font-size: 12px;
		font-weight: 600;
		box-shadow: 0 4px 16px rgba(15, 23, 42, 0.12);
		cursor: pointer;
		z-index: 10;
		backdrop-filter: blur(6px);
	}

	.jump-to-bottom:hover {
		background: rgba(255, 255, 255, 1);
	}
</style>
