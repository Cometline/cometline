<script lang="ts">
	import { Search, SquarePen } from '@lucide/svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';

	let {
		searchQuery = $bindable(''),
		searchInput = $bindable<HTMLInputElement | null>(null),
		onNewChat
	}: {
		searchQuery?: string;
		searchInput?: HTMLInputElement | null;
		onNewChat: () => void;
	} = $props();
</script>

<div class="search-field-wrap no-drag">
	<div class="search-composite">
		<label class="search-field">
			<Search size={14} stroke-width={2} aria-hidden="true" class="search-icon" />
			<input
				type="search"
				class="search-input"
				placeholder="Search chats"
				bind:value={searchQuery}
				bind:this={searchInput}
				spellcheck="false"
				autocomplete="off"
				aria-label="Search chats by title"
			/>
		</label>
		<div class="search-divider" aria-hidden="true"></div>
		<Tooltip label="New chat" action="newChat">
			<button class="new-chat-button" onclick={onNewChat} aria-label="New chat">
				<SquarePen size={16} stroke-width={1.8} />
			</button>
		</Tooltip>
	</div>
</div>

<style>
	.search-field-wrap {
		flex: 1;
		min-width: 0;
	}

	.search-composite {
		display: flex;
		align-items: stretch;
		min-width: 0;
		height: 1.75rem;
		border: 1px solid var(--border-soft);
		border-radius: 0.5rem;
		background: rgba(255, 255, 255, 0.7);
		color: var(--text-soft);
	}

	.search-composite:focus-within {
		border-color: rgba(15, 23, 42, 0.15);
		background: rgba(255, 255, 255, 0.95);
		color: var(--text-muted);
	}

	.search-field {
		display: flex;
		min-width: 0;
		flex: 1;
		align-items: center;
		gap: 0.375rem;
		padding: 0 0.375rem 0 0.625rem;
	}

	.search-field :global(.search-icon) {
		flex-shrink: 0;
	}

	.search-input {
		min-width: 0;
		flex: 1;
		width: 100%;
		border: 0;
		background: transparent;
		padding: 0 1px 0 0;
		font-size: 0.75rem;
		color: var(--text-main);
		outline: none;
		overflow: visible;
	}

	.search-input::-webkit-search-decoration,
	.search-input::-webkit-search-cancel-button,
	.search-input::-webkit-search-results-button {
		display: none;
	}

	.search-input::placeholder {
		color: var(--text-soft);
	}

	.search-divider {
		width: 1px;
		background: var(--border-soft);
	}

	.search-composite :global(.tooltip-wrap) {
		align-self: stretch;
		height: 100%;
	}

	.new-chat-button {
		display: grid;
		width: 1.625rem;
		height: 100%;
		flex-shrink: 0;
		place-items: center;
		border: 0;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.new-chat-button:hover {
		color: var(--text-main);
	}
</style>
