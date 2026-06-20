<script lang="ts">
	import { slide } from 'svelte/transition';
	import {
		Terminal,
		ChevronDown,
		LoaderCircle,
		TriangleAlert,
		CircleCheck
	} from '@lucide/svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	const FOLD_IN = { duration: 180 };

	let {
		item,
		label,
		expanded,
		onToggle
	}: {
		item: Extract<ChatItem, { type: 'tool' }>;
		label: string;
		expanded: boolean;
		onToggle: () => void;
	} = $props();

	function formatToolInput(input: unknown) {
		if (input == null) return '';
		if (typeof input === 'string') return input.trim();
		try {
			return JSON.stringify(input, null, 2);
		} catch {
			return String(input);
		}
	}
</script>

<div class="fold-panel tool-fold-panel" class:error={!!item.error}>
	<button
		type="button"
		class="fold-toggle tool-fold-toggle"
		aria-expanded={expanded}
		onclick={onToggle}
	>
		<Terminal size={13} />
		<span>{label}</span>
		{#if item.pending}
			<LoaderCircle size={12} class="spin" />
		{:else if item.error}
			<TriangleAlert size={12} />
		{:else}
			<CircleCheck size={12} />
		{/if}
		<ChevronDown size={13} class={expanded ? 'expanded' : ''} />
	</button>
	{#if expanded}
		<div class="fold-body tool-output-body" transition:slide={FOLD_IN}>
			{#if formatToolInput(item.input)}
				<pre class="tool-input-text">{formatToolInput(item.input)}</pre>
			{/if}
			{#if item.error}
				<pre class="tool-error-text">{item.error}</pre>
			{:else if item.output}
				<pre>{item.output}</pre>
			{/if}
			{#if item.pending && !item.output && !item.error}
				<pre>Running…</pre>
			{/if}
		</div>
	{/if}
</div>

<style>
	.fold-panel {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.tool-fold-panel.error .tool-fold-toggle {
		border-color: rgba(239, 68, 68, 0.35);
		color: #b91c1c;
	}

	.tool-input-text {
		margin: 0 0 8px;
		font-size: 11px;
		color: var(--text-muted);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.fold-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		align-self: flex-start;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.72);
		color: var(--text-muted);
		border-radius: 999px;
		padding: 5px 10px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
	}

	.fold-toggle:hover {
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-main);
	}

	.fold-toggle :global(svg.expanded) {
		transform: rotate(180deg);
	}

	.tool-output-body {
		margin-top: 8px;
		border: 1px solid var(--border-soft);
		background: rgba(15, 23, 42, 0.03);
		border-radius: 10px;
		padding: 8px 10px;
		color: var(--text-muted);
	}

	.tool-output-body pre {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		font-family: inherit;
		white-space: pre-wrap;
		word-break: normal;
		overflow-wrap: break-word;
		max-height: 220px;
		overflow: auto;
		scrollbar-gutter: stable;
	}

	.tool-output-body pre + pre {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	.tool-error-text {
		color: #b42318;
	}
</style>
