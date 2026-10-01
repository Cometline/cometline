<script lang="ts">
	import { fade } from 'svelte/transition';
	import { FileText } from '@lucide/svelte';

	let {
		dragActive,
		dropProcessing,
		dropMessage
	}: {
		dragActive: boolean;
		dropProcessing: boolean;
		dropMessage: string;
	} = $props();
</script>

{#if dragActive}
	<div class="drop-overlay" aria-hidden="true">
		<FileText size={18} stroke-width={1.8} />
		<span>{dropProcessing ? 'Reading files…' : 'Drop text files to add context'}</span>
	</div>
{/if}

{#if dropMessage}
	<div class="drop-message" role="status" transition:fade={{ duration: 120 }}>
		{dropMessage}
	</div>
{/if}

<style>
	.drop-overlay {
		position: absolute;
		inset: 8px;
		z-index: 20;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		border: 1px dashed rgba(37, 99, 235, 0.34);
		border-radius: calc(var(--radius-card) - 6px);
		background: rgba(255, 255, 255, 0.78);
		color: var(--color-1d4ed8);
		font-size: 13px;
		font-weight: 600;
		pointer-events: none;
		backdrop-filter: blur(10px);
		-webkit-backdrop-filter: blur(10px);
	}

	.drop-message {
		position: absolute;
		right: 12px;
		bottom: calc(100% + 8px);
		z-index: 25;
		max-width: min(360px, calc(100vw - 32px));
		padding: 7px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.96);
		box-shadow: var(--shadow-card);
		color: var(--text-muted);
		font-size: 12px;
		line-height: 1.35;
	}
</style>
