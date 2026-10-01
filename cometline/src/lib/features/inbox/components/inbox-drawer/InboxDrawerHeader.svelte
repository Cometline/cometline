<script lang="ts">
	import { Bell, X } from '@lucide/svelte';

	let { count, onClose }: { count: number; onClose: () => void } = $props();
</script>

<header class="inbox-header">
	<div class="inbox-title">
		<span class="title-mark" aria-hidden="true">
			<Bell size={16} stroke-width={1.8} />
		</span>
		<div class="title-copy">
			<h2>Inbox</h2>
			{#if count > 0}
				<p>
					{`${count} open message${count === 1 ? '' : 's'}`}
				</p>
			{/if}
		</div>
	</div>
	<button type="button" class="icon-btn" aria-label="Close" onclick={onClose}>
		<X size={16} stroke-width={1.8} />
	</button>
</header>

<style>
	.inbox-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 16px 18px 14px;
		border-bottom: 1px solid var(--border-soft);
		flex-shrink: 0;
	}

	.inbox-title {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
	}

	.title-copy {
		display: flex;
		flex-direction: column;
		justify-content: center;
		min-height: 32px;
		min-width: 0;
	}

	.title-mark {
		display: grid;
		place-items: center;
		width: 32px;
		height: 32px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--accent, var(--text-main)) 12%, transparent);
		color: var(--accent, var(--text-main));
		flex-shrink: 0;
	}

	.inbox-title h2 {
		margin: 0;
		font-size: 16px;
		font-weight: 650;
		line-height: 1.25;
		color: var(--text-main);
	}

	.inbox-title p {
		margin: 2px 0 0;
		font-size: 12px;
		color: var(--text-muted, var(--text-soft));
	}

	.icon-btn {
		width: 30px;
		height: 30px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-muted, var(--text-soft));
		display: grid;
		place-items: center;
		cursor: pointer;
		flex-shrink: 0;
	}

	.icon-btn:hover {
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
		color: var(--text-main);
	}
</style>
