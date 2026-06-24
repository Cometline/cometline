<script lang="ts">
	import { slide } from 'svelte/transition';
	import { Brain, ChevronDown } from '@lucide/svelte';
	import ThinkingSpinner from '$lib/components/ThinkingSpinner.svelte';

	const FOLD_IN = { duration: 180 };

	let {
		text,
		pending: _pending = false,
		expanded,
		showSpinner = false,
		onToggle,
		nested = false,
		contentOnly = false,
		toggleDisabled = false
	}: {
		text: string;
		pending?: boolean;
		expanded: boolean;
		showSpinner?: boolean;
		onToggle: () => void;
		nested?: boolean;
		contentOnly?: boolean;
		toggleDisabled?: boolean;
	} = $props();

	function autoScrollBottom(node: HTMLElement, value: string) {
		const pin = (content: string) => {
			void content;
			node.scrollTop = node.scrollHeight;
		};
		pin(value);
		return {
			update(content: string) {
				pin(content);
			}
		};
	}
</script>

<div class="fold-panel thinking-panel" class:nested class:content-only={contentOnly}>
	{#if !contentOnly}
		<button
			type="button"
			class="fold-toggle thinking-toggle"
			aria-expanded={expanded && !toggleDisabled}
			disabled={toggleDisabled}
			onclick={onToggle}
		>
			<Brain size={13} />
			<span>Thinking</span>
			{#if showSpinner}
				<ThinkingSpinner size={12} label="Thinking" />
			{/if}
			<ChevronDown size={13} class={expanded && !toggleDisabled ? 'expanded' : ''} />
		</button>
	{/if}
	{#if expanded && !toggleDisabled}
		<div class="fold-body thinking-body" transition:slide={FOLD_IN}>
			<div class="thinking-reasoning">
				<p class="scrollbar-none" use:autoScrollBottom={text}>
					{text || 'Thinking…'}
				</p>
			</div>
		</div>
	{/if}
</div>

<style>
	/* Base .fold-panel / .fold-toggle / .fold-body styles live in
	   src/lib/styles/fold-panel.css. Only component-specific overrides here. */
	.fold-panel.nested {
		align-self: stretch;
	}

	.fold-panel.nested .fold-toggle {
		align-self: stretch;
	}

	.thinking-body {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.thinking-reasoning p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 220px;
		overflow: auto;
		color: var(--text-muted);
	}
</style>
