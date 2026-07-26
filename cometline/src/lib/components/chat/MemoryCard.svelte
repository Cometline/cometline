<script lang="ts">
	import { fade, slide } from 'svelte/transition';
	import { Brain, ChevronDown } from '@lucide/svelte';
	import type { InjectedMemory } from '$lib/conversation/thinking-attribution';
	import { bucketMemories, memoryKindLabel, resolveMemoryBucket } from '$lib/memory/buckets';

	const FOLD_IN = { duration: 180 };
	const CHIP_FADE = { duration: 400 };

	let {
		memories,
		expanded,
		onToggle,
		nested = false,
		contentOnly = false,
		cycling = false,
		cycleTick
	}: {
		memories: InjectedMemory[];
		expanded: boolean;
		onToggle: () => void;
		nested?: boolean;
		contentOnly?: boolean;
		cycling?: boolean;
		cycleTick?: number;
	} = $props();

	let internalCycleTick = $state(0);
	let activeCycleTick = $derived(cycleTick ?? internalCycleTick);
	let sections = $derived(bucketMemories(memories));

	$effect(() => {
		if (!cycling || cycleTick !== undefined) return;
		const timer = setInterval(() => internalCycleTick++, 5000);
		return () => clearInterval(timer);
	});
</script>

{#snippet memoryBodyContent()}
	{#if cycling && memories.length > 0}
		{#key activeCycleTick}
			{@const mem = memories[activeCycleTick % memories.length]}
			{@const bucket = resolveMemoryBucket(mem)}
			{@const section = sections.find((candidate) => candidate.bucket === bucket)}
			<div class="memory-chip-cycling-wrap">
				<div class="memory-section memory-chip-cycling" in:fade={CHIP_FADE}>
					<div class="memory-section-title">{section?.label}</div>
					<div class="memory-chip" title={mem.content}>
						{#if bucket === 'semantic'}
							<span class="memory-kind">{memoryKindLabel(mem.kind)}</span>
						{/if}
						<span class="memory-content">{mem.content}</span>
					</div>
				</div>
			</div>
		{/key}
	{:else}
		<div class="memory-sections">
			{#each sections as section (section.bucket)}
				<section class="memory-section" aria-label={section.label}>
					<div class="memory-section-title">{section.label}</div>
					<div class="memory-chips">
						{#each section.memories as mem (mem.id)}
							<div class="memory-chip" title={mem.content}>
								{#if section.bucket === 'semantic'}
									<span class="memory-kind">{memoryKindLabel(mem.kind)}</span>
								{/if}
								<span class="memory-content">{mem.content}</span>
							</div>
						{/each}
					</div>
				</section>
			{/each}
		</div>
	{/if}
{/snippet}

<div class="fold-panel memory-panel" class:nested class:content-only={contentOnly}>
	{#if !contentOnly}
		<button
			type="button"
			class="fold-toggle memory-toggle"
			aria-expanded={expanded}
			onclick={onToggle}
		>
			<Brain size={13} />
			<span>Memories used · {memories.length}</span>
			<ChevronDown size={13} class={expanded ? 'expanded' : ''} />
		</button>
	{/if}
	{#if contentOnly}
		<div class="fold-body memory-body">
			{@render memoryBodyContent()}
		</div>
	{:else if expanded}
		<div class="fold-body memory-body" transition:slide={FOLD_IN}>
			{@render memoryBodyContent()}
		</div>
	{/if}
</div>

<style>
	/* Base .fold-panel / .fold-toggle / .fold-body styles live in
	   src/lib/styles/fold-panel.css. Only component-specific overrides here. */
	.memory-panel {
		min-width: 0;
		max-width: 100%;
	}

	.fold-panel.nested {
		align-self: stretch;
	}

	.fold-panel.nested .fold-toggle {
		align-self: stretch;
	}

	.fold-panel.content-only .memory-body {
		border: none;
		background: transparent;
		padding: 0;
	}

	.memory-body {
		width: 100%;
		box-sizing: border-box;
		min-width: 0;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 8px 10px;
		background: rgba(0, 102, 204, 0.04);
	}

	.memory-sections,
	.memory-section,
	.memory-chips {
		display: flex;
		min-width: 0;
		flex-direction: column;
	}

	.memory-sections {
		gap: 10px;
	}

	.memory-section,
	.memory-chips {
		gap: 6px;
	}

	.memory-section-title {
		color: var(--text-muted);
		font-size: 10px;
		font-weight: 650;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}

	.memory-chip {
		display: flex;
		align-items: baseline;
		gap: 7px;
		width: 100%;
		min-width: 0;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		padding: 5px 10px;
		border-radius: 10px;
		background: rgba(0, 102, 204, 0.08);
		color: var(--text-main);
		font-size: 11px;
		line-height: 1.45;
	}

	.memory-kind {
		flex: 0 0 auto;
		color: var(--accent);
		font-size: 9px;
		font-weight: 650;
		text-transform: capitalize;
	}

	.memory-content {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.memory-chip-cycling-wrap {
		display: grid;
		min-width: 0;
		width: 100%;
	}

	.memory-chip-cycling {
		grid-column: 1;
		grid-row: 1;
	}
</style>
