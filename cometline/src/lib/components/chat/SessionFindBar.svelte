<script lang="ts">
	import { tick } from 'svelte';
	import { ChevronDown, ChevronUp, Search, X } from '@lucide/svelte';
	import type { SessionFindController } from '$lib/conversation/session-find.svelte';

	let { controller }: { controller: SessionFindController } = $props();
	let input = $state<HTMLInputElement | null>(null);
	const resultPosition = $derived(controller.activeIndex >= 0 ? controller.activeIndex + 1 : 0);

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			event.stopPropagation();
			if (event.shiftKey) controller.previous();
			else controller.next();
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
			controller.closeFind();
		}
	}

	$effect(() => {
		const request = controller.focusRequestId;
		if (!controller.open) return;
		void tick().then(() => {
			if (request !== controller.focusRequestId) return;
			input?.focus();
			input?.select();
		});
	});
</script>

<div class="session-find-bar" role="search" aria-label="Find in current chat">
	<Search size={15} aria-hidden="true" />
	<input
		bind:this={input}
		type="search"
		value={controller.query}
		placeholder="Find in chat"
		aria-label="Find text in current chat"
		oninput={(event) => controller.setQuery(event.currentTarget.value)}
		onkeydown={onKeydown}
	/>
	<span class="result-count" aria-live="polite">
		{resultPosition} / {controller.matchCount}
	</span>
	<button
		type="button"
		class="find-action"
		aria-label="Previous match"
		disabled={controller.matchCount === 0}
		onclick={controller.previous}
	>
		<ChevronUp size={15} />
	</button>
	<button
		type="button"
		class="find-action"
		aria-label="Next match"
		disabled={controller.matchCount === 0}
		onclick={controller.next}
	>
		<ChevronDown size={15} />
	</button>
	<button
		type="button"
		class="find-action"
		aria-label="Close find"
		onclick={() => controller.closeFind()}
	>
		<X size={15} />
	</button>
</div>

<style>
	.session-find-bar {
		position: absolute;
		top: 10px;
		right: max(10px, var(--chat-gutter));
		z-index: 12;
		display: flex;
		align-items: center;
		gap: 5px;
		width: min(370px, calc(100% - 20px));
		box-sizing: border-box;
		padding: 7px 8px 7px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: color-mix(in srgb, var(--panel-bg) 96%, white);
		box-shadow: 0 10px 30px rgba(15, 23, 42, 0.14);
		color: var(--text-soft);
	}

	input {
		min-width: 0;
		flex: 1;
		border: 0;
		outline: 0;
		background: transparent;
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
	}

	input::placeholder {
		color: var(--text-soft);
	}

	.result-count {
		flex: 0 0 auto;
		min-width: 48px;
		font-size: 11px;
		font-variant-numeric: tabular-nums;
		text-align: right;
		color: var(--text-muted);
	}

	.find-action {
		display: grid;
		place-items: center;
		width: 26px;
		height: 26px;
		padding: 0;
		border: 0;
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.find-action:hover:not(:disabled) {
		background: color-mix(in srgb, var(--border-soft) 45%, transparent);
		color: var(--text-main);
	}

	.find-action:disabled {
		opacity: 0.35;
		cursor: default;
	}

	.find-action:focus-visible,
	input:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}

	@media (max-width: 480px) {
		.session-find-bar {
			right: 8px;
			width: calc(100% - 16px);
		}
	}
</style>
