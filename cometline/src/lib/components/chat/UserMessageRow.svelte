<script lang="ts">
	import { Check, Copy } from '@lucide/svelte';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import ThreadAvatar from '$lib/components/chat/ThreadAvatar.svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import { imageDataURL } from '$lib/files/images';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	let {
		item,
		avatarSrc,
		avatarSrcset,
		continuationRow = false,
		copiedId,
		onCopyMessage
	}: {
		item: Extract<ChatItem, { type: 'user' }>;
		avatarSrc: string;
		avatarSrcset?: string;
		continuationRow?: boolean;
		copiedId: string | null;
		onCopyMessage: (id: string, text: string) => void | Promise<void>;
	} = $props();
</script>

<ThreadRow variant="user" {continuationRow} data-user-item-id={item.id}>
	<ThreadAvatar variant="gutter" {avatarSrc} {avatarSrcset} />
	<div class="user-stack">
		<div
			class="bubble user-bubble"
			class:flight-hidden={item.reveal === false}
			data-flight-target={item.reveal === false ? 'user' : undefined}
		>
			{#if item.images?.length}
				<div class="user-images" class:text-following={Boolean(item.text)}>
					{#each item.images as image, imageIndex (`${item.id}-image-${image.id ?? imageIndex}`)}
						<img src={imageDataURL(image)} alt={image.name ?? 'Attached image'} />
					{/each}
				</div>
			{/if}
			{#if item.text?.trim()}
				<AssistantMarkdown source={item.text.trim()} mode="user" />
			{/if}
		</div>
		{#if item.text?.trim()}
			<div class="message-actions user-message-actions">
				<button
					type="button"
					class="message-action m-1"
					class:copied={copiedId === item.id}
					title="Copy message"
					aria-label="Copy message"
					onclick={() => onCopyMessage(item.id, item.text.trim())}
				>
					{#if copiedId === item.id}
						<Check size={13} />
						<span>Copied</span>
					{:else}
						<Copy size={13} />
						<span>Copy</span>
					{/if}
				</button>
			</div>
		{/if}
	</div>
</ThreadRow>

<style>
	.user-stack {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex: 1 1 auto;
		min-width: 0;
		max-width: var(--chat-assistant-column);
	}

	.flight-hidden {
		opacity: 0;
		pointer-events: none;
	}

	.message-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-top: -2px;
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	.user-stack:hover .message-actions,
	.message-actions:focus-within {
		opacity: 1;
	}

	.user-message-actions {
		justify-content: flex-end;
	}

	.message-action {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 8px;
		border: 1px solid transparent;
		border-radius: 7px;
		background: transparent;
		color: var(--text-soft);
		font-size: 11px;
		font-weight: 600;
		line-height: 1;
		cursor: pointer;
		transition:
			color var(--duration-fast) var(--ease-smooth),
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	.message-action:hover {
		color: var(--text-main);
		background: rgba(255, 255, 255, 0.92);
		border-color: var(--border-soft);
	}

	.message-action.copied {
		color: var(--status-success);
	}

	.message-action :global(svg) {
		flex-shrink: 0;
	}

	@media (prefers-reduced-motion: reduce) {
		.message-actions {
			transition: none;
		}
	}
</style>
