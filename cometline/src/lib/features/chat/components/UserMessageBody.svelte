<script lang="ts">
	import AssistantMarkdown from '#lib/components/AssistantMarkdown.svelte';
	import PastedTextChip from '#lib/features/chat/components/PastedTextChip.svelte';
	import {
		parsePastedDisplay,
		shouldCollapsePastedText
	} from '#lib/features/composer/rich-composer-paste.js';

	let { text }: { text: string } = $props();

	// Display documents keep each paste chip and the typed text as siblings.
	// A raw oversized string has no boundary left, so it stays one chip.
	let parts = $derived(parsePastedDisplay(text));
	let collapse = $derived(!parts && shouldCollapsePastedText(text));
</script>

{#if parts}
	<span class="user-mixed">
		{#each parts as part, index (index)}
			{#if part.kind === 'paste'}
				<PastedTextChip label={part.label} />
			{:else if part.text}
				<span class="user-mixed-text">{part.text}</span>
			{/if}
		{/each}
	</span>
{:else if collapse}
	<PastedTextChip {text} />
{:else}
	<AssistantMarkdown source={text} mode="user" />
{/if}

<style>
	.user-mixed {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.user-mixed-text {
		white-space: pre-wrap;
	}

	.user-mixed :global(.pasted-text-chip) {
		vertical-align: middle;
		margin-right: 0.2em;
	}
</style>
