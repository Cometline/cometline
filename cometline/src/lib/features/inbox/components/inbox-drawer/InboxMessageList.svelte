<script lang="ts">
	import type { InboxMessageResource } from '#lib/client/cometmind.js';
	import { formatRelativeTime, previewSnippet } from '#lib/features/inbox/inbox-drawer-format.js';

	let {
		messages,
		selectedId,
		onSelect
	}: {
		messages: InboxMessageResource[];
		selectedId: string | undefined;
		onSelect: (id: string) => void;
	} = $props();
</script>

<ul class="inbox-list" aria-label="Inbox messages">
	{#each messages as message (message.id)}
		<li>
			<button
				type="button"
				class="inbox-row"
				class:selected={selectedId === message.id}
				onclick={() => onSelect(message.id)}
			>
				<div class="row-top">
					<span class="row-title">{message.title}</span>
					<span class="row-time">{formatRelativeTime(message.created_at)}</span>
				</div>
				<span class="row-preview">{previewSnippet(message.body)}</span>
			</button>
		</li>
	{/each}
</ul>

<style>
	.inbox-list {
		list-style: none;
		margin: 0;
		padding: 8px;
		overflow-y: auto;
		min-height: 0;
		border-bottom: 1px solid var(--border-soft);
	}

	:global(.inbox-modal.has-selection) .inbox-list {
		border-bottom: none;
		border-right: 1px solid var(--border-soft);
	}

	.inbox-row {
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 10px 12px;
		border: none;
		border-radius: 10px;
		background: transparent;
		text-align: left;
		cursor: pointer;
		color: var(--text-main);
	}

	.inbox-row:hover,
	.inbox-row.selected {
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
	}

	.row-top {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 8px;
	}

	.row-title {
		font-size: 13px;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.row-time {
		flex-shrink: 0;
		font-size: 11px;
		color: var(--text-muted, var(--text-soft));
	}

	.row-preview {
		font-size: 12px;
		line-height: 1.35;
		color: var(--text-soft, var(--text-muted));
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	@media (max-width: 640px) {
		:global(.inbox-modal.has-selection) .inbox-list {
			border-right: none;
			border-bottom: 1px solid var(--border-soft);
		}
	}
</style>
