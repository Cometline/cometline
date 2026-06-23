<script lang="ts">
	import { fly } from 'svelte/transition';
	import { ChevronDown, X } from '@lucide/svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';

	let {
		queuedCount,
		queuedMessages,
		onRemove
	}: {
		queuedCount: number;
		queuedMessages: QueuedMessage[];
		onRemove: (id: string) => void;
	} = $props();

	let queuePreviewOpen = $state(false);
	let queuePicker = $state<HTMLDivElement | null>(null);

	function toggleQueuePreview() {
		queuePreviewOpen = !queuePreviewOpen;
	}

	export function closePreview() {
		queuePreviewOpen = false;
	}
</script>

{#if queuedCount > 0}
	<div
		class="queue-picker"
		bind:this={queuePicker}
		in:fly={{ y: 4, duration: 140 }}
		out:fly={{ y: 4, duration: 120 }}
	>
		<button
			type="button"
			class="queue-banner"
			class:open={queuePreviewOpen}
			aria-live="polite"
			aria-expanded={queuePreviewOpen}
			aria-controls="queue-preview-panel"
			onclick={toggleQueuePreview}
		>
			<span>{queuedCount} {queuedCount === 1 ? 'message' : 'messages'} queued</span>
			<ChevronDown size={12} class={queuePreviewOpen ? 'expanded' : ''} />
		</button>

		{#if queuePreviewOpen}
			<div
				id="queue-preview-panel"
				class="queue-preview"
				role="region"
				aria-label="Queued messages"
				transition:fly={{ y: -4, duration: 120 }}
			>
				<ul class="queue-preview-list scrollbar-none">
					{#each queuedMessages as message, index (message.id)}
						<li class="queue-preview-item">
							<span class="queue-preview-index">{index + 1}</span>
							<p class="queue-preview-text">{message.text}</p>
							<button
								type="button"
								class="queue-remove"
								aria-label={`Remove queued message ${index + 1}`}
								onpointerdown={(e) => {
									e.preventDefault();
									e.stopPropagation();
									onRemove(message.id);
								}}
							>
								<X size={12} stroke-width={2} />
							</button>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	</div>
{/if}

<style>
	.queue-picker {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.queue-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		margin: -2px 0 -4px;
		padding: 6px 10px;
		border: none;
		border-radius: 10px;
		background: rgba(15, 23, 42, 0.04);
		font-size: 11px;
		font-weight: 500;
		line-height: 1.2;
		color: var(--text-soft);
		cursor: pointer;
		text-align: left;
	}

	.queue-banner:hover,
	.queue-banner.open {
		background: rgba(15, 23, 42, 0.07);
		color: var(--text-muted);
	}

	.queue-banner :global(svg) {
		flex-shrink: 0;
		transition: transform var(--duration-fast) var(--ease-smooth);
	}

	.queue-banner :global(.expanded) {
		transform: rotate(180deg);
	}

	.queue-preview {
		padding: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.92);
		box-shadow: var(--shadow-card);
	}

	.queue-preview-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-height: 160px;
		overflow-y: auto;
	}

	.queue-preview-item {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 7px 8px;
		border-radius: 8px;
		background: rgba(15, 23, 42, 0.03);
	}

	.queue-remove {
		flex: 0 0 auto;
		display: grid;
		place-items: center;
		margin-left: auto;
		padding: 4px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-soft);
		cursor: pointer;
	}

	.queue-remove:hover {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.queue-preview-index {
		flex: 0 0 auto;
		font-size: 10px;
		font-weight: 700;
		color: var(--text-soft);
		padding-top: 2px;
	}

	.queue-preview-text {
		margin: 0;
		min-width: 0;
		flex: 1;
		font-size: 11px;
		line-height: 1.4;
		color: var(--text-main);
		overflow-wrap: anywhere;
	}
</style>
