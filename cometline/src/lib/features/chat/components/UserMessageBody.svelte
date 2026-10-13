<script lang="ts">
	import AssistantMarkdown from '#lib/components/AssistantMarkdown.svelte';
	import { parsePastedDisplay } from '#lib/features/composer/rich-composer-paste.js';

	let { text }: { text: string } = $props();

	// Only text that left the composer as a paste chip carries a display token.
	let parts = $derived(parsePastedDisplay(text));
</script>

{#if parts}
	<span class="user-mixed">
		{#each parts as part, index (index)}
			{#if part.kind === 'paste'}
				<span class="pasted-text-chip">{part.label}</span>
			{:else if part.text}
				{part.text}
			{/if}
		{/each}
	</span>
{:else}
	<AssistantMarkdown source={text} mode="user" />
{/if}

<style>
	.user-mixed {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.pasted-text-chip {
		display: inline-flex;
		max-width: 16rem;
		margin-right: 0.2em;
		padding: 0.1em 0.45em;
		border: 1px solid rgba(255, 255, 255, 0.28);
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.14);
		font-size: 0.92em;
		font-weight: 650;
		line-height: 1.3;
		vertical-align: middle;
		white-space: nowrap;
		user-select: none;
	}
</style>
