<script lang="ts">
	import { FileText, Folder, Loader } from '@lucide/svelte';
	import SlashCommandMenu from '$lib/components/composer/SlashCommandMenu.svelte';
	import type { createComposerMentionsController } from '$lib/components/composer/composer-mentions.svelte';

	type MentionsController = ReturnType<typeof createComposerMentionsController>;

	let {
		mentions,
		menuRef = $bindable<HTMLDivElement | null>(null)
	}: {
		mentions: MentionsController;
		menuRef?: HTMLDivElement | null;
	} = $props();

	const mentionResults = $derived(mentions.filteredMentionFiles ?? []);
	const mentionQueryText = $derived((mentions.mentionQuery ?? '').trim());
	const mentionSearching = $derived(
		mentions.useServerSearch && (mentions.mentionServerLoading || mentions.mentionSearchPending)
	);
</script>

{#if mentions.mentionMenuOpen}
	<SlashCommandMenu ariaLabel="Files and folders" class="mention-menu" bind:menuRef>
		{#if !mentions.fileIndexReady && mentionResults.length === 0}
			<p class="skill-command-loading">
				<Loader size={13} stroke-width={2} class="mention-spinner" />
				<span>Indexing files…</span>
			</p>
		{:else if mentionSearching && mentionResults.length === 0}
			<p class="skill-command-loading">
				<Loader size={13} stroke-width={2} class="mention-spinner" />
				<span>Searching…</span>
			</p>
		{:else if mentions.fileIndex?.error && mentionResults.length === 0}
			<p class="skill-command-empty">Could not index files.</p>
		{:else if mentionResults.length === 0 && mentionQueryText}
			<p class="skill-command-empty">File not found.</p>
		{:else if mentionResults.length === 0}
			<p class="skill-command-empty">No matching files.</p>
		{:else}
			{#each mentionResults as item, index (item.kind + ':' + item.path)}
				<button
					type="button"
					class="skill-command-option mention-option"
					class:highlighted={index === mentions.mentionHighlight}
					data-mention-index={index}
					role="option"
					aria-selected={index === mentions.mentionHighlight}
					onpointerenter={() => (mentions.mentionHighlight = index)}
					onpointerdown={(e) => {
						e.preventDefault();
						mentions.selectMentionFile(item);
					}}
				>
					{#if item.kind === 'dir'}
						<Folder size={14} stroke-width={1.8} />
					{:else}
						<FileText size={14} stroke-width={1.8} />
					{/if}
					<span class="mention-path">{item.path}</span>
				</button>
			{/each}
		{/if}
		{#if mentions.mentionTruncated && !mentionQueryText}
			<p class="mention-hint">Type to search more files in the workspace.</p>
		{/if}
	</SlashCommandMenu>
{/if}
