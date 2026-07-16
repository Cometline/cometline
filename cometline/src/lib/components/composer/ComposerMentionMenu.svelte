<script lang="ts">
	import { FileText, Loader } from '@lucide/svelte';
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
</script>

{#if mentions.mentionMenuOpen}
	<SlashCommandMenu ariaLabel="Files" class="mention-menu" bind:menuRef>
		{#if !mentions.fileIndexReady && mentions.filteredMentionFiles.length === 0}
			<p class="skill-command-loading">
				<Loader size={13} stroke-width={2} class="mention-spinner" />
				<span>Indexing files…</span>
			</p>
		{:else if mentions.useServerSearch && mentions.mentionServerLoading && mentions.filteredMentionFiles.length === 0}
			<p class="skill-command-loading">
				<Loader size={13} stroke-width={2} class="mention-spinner" />
				<span>Searching…</span>
			</p>
		{:else if mentions.fileIndex?.error && !mentions.wikiFileIndex.loaded && mentions.filteredMentionFiles.length === 0}
			<p class="skill-command-empty">Could not index files.</p>
		{:else if mentions.filteredMentionFiles.length === 0}
			<p class="skill-command-empty">No matching files.</p>
		{:else}
			{#each mentions.filteredMentionFiles as option, index (`${option.source}:${option.path}`)}
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
						mentions.selectMentionFile(option);
					}}
				>
					<FileText size={14} stroke-width={1.8} />
					<span class="mention-path">{option.label}</span>
					<span class="mention-source">{option.source === 'wiki' ? 'Wiki' : 'Workspace'}</span>
				</button>
			{/each}
		{/if}
		{#if mentions.mentionTruncated && !mentions.mentionQuery.trim()}
			<p class="mention-hint">Type to search more files in the wiki or workspace.</p>
		{/if}
	</SlashCommandMenu>
{/if}

<style>
	.mention-source {
		margin-left: auto;
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--text-muted);
	}
</style>
