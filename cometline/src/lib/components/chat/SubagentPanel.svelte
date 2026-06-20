<script lang="ts">
	import { slide } from 'svelte/transition';
	import {
		Terminal,
		ChevronDown,
		LoaderCircle,
		TriangleAlert,
		CircleCheck,
		CircleX
	} from '@lucide/svelte';
	import { chatStore } from '$lib/stores/chat.svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	const FOLD_IN = { duration: 180 };

	let {
		item,
		expanded,
		onToggle
	}: {
		item: Extract<ChatItem, { type: 'subagent' }>;
		expanded: boolean;
		onToggle: () => void;
	} = $props();

	function subagentProgressLabel(subagent: Extract<ChatItem, { type: 'subagent' }>) {
		const toolCount = subagent.progress.filter((entry) => entry.kind === 'tool').length;
		const prefix =
			subagent.status === 'failed'
				? 'OpenCode failed'
				: subagent.status === 'cancelled'
					? 'OpenCode cancelled'
					: `OpenCode · ${subagent.agentName}`;
		if (toolCount > 0) {
			return `${prefix} · ${toolCount} tool${toolCount === 1 ? '' : 's'}`;
		}
		return prefix;
	}

	function subagentVisibleProgress(subagent: Extract<ChatItem, { type: 'subagent' }>) {
		if (subagent.status === 'running') {
			return subagent.progress.filter(
				(entry) => !(entry.kind === 'stream' && entry.channel === 'message')
			);
		}
		return subagent.progress;
	}
</script>

<div class="fold-panel subagent-panel" class:pending={item.pending}>
	<div class="subagent-header">
		<button
			type="button"
			class="fold-toggle subagent-toggle"
			aria-expanded={expanded}
			onclick={onToggle}
		>
			<Terminal size={13} />
			<span>{subagentProgressLabel(item)}</span>
			{#if item.pending}
				<LoaderCircle size={12} class="spin" />
			{:else if item.status === 'failed'}
				<TriangleAlert size={12} />
			{:else if item.status === 'cancelled'}
				<CircleX size={12} />
			{:else}
				<CircleCheck size={12} />
			{/if}
			<ChevronDown size={13} class={expanded ? 'expanded' : ''} />
		</button>
		{#if item.pending}
			<button
				type="button"
				class="subagent-cancel"
				onclick={() => chatStore.cancelSubagent(item.childSessionId)}
			>
				Cancel
			</button>
		{/if}
	</div>
	{#if expanded}
		{@const visibleProgress = subagentVisibleProgress(item)}
		<div class="fold-body subagent-body" transition:slide={FOLD_IN}>
			<p class="subagent-purpose">{item.purpose}</p>
			{#if visibleProgress.length > 0}
				<div class="subagent-progress">
					{#each visibleProgress as entry, entryIndex (`${item.id}-progress-${entry.kind}-${entryIndex}`)}
						{#if entry.kind === 'tool'}
							<div
								class="subagent-tool"
								class:pending={entry.status === 'pending' ||
									entry.status === 'in_progress'}
							>
								<div class="subagent-tool-header">
									<Terminal size={12} />
									<span class="subagent-tool-name">{entry.title}</span>
									{#if entry.status}
										<span class="subagent-tool-status">{entry.status}</span>
									{/if}
								</div>
							</div>
						{:else if entry.text.trim()}
							<p
								class="subagent-stream"
								class:subagent-thought={entry.channel === 'thought'}
								class:subagent-plan={entry.channel === 'plan'}
							>
								{entry.text}
							</p>
						{/if}
					{/each}
				</div>
			{/if}
			{#if item.summary}
				<div class="subagent-summary">
					<p>{item.summary}</p>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.fold-panel {
		display: flex;
		flex-direction: column;
		gap: 6px;
		width: 100%;
		min-width: 0;
	}

	.subagent-panel.pending .subagent-toggle {
		border-color: rgba(59, 130, 246, 0.22);
	}

	.subagent-header {
		display: flex;
		align-items: stretch;
		gap: 8px;
		min-width: 0;
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

	.subagent-toggle {
		min-width: 0;
		flex: 1;
	}

	.fold-toggle:hover {
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-main);
	}

	.fold-toggle :global(svg.expanded) {
		transform: rotate(180deg);
	}

	.subagent-cancel {
		flex: 0 0 auto;
		border: 1px solid var(--border-soft);
		border-radius: 999px;
		padding: 0 12px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		background: var(--surface-muted, var(--bg-muted));
		color: var(--text-muted);
	}

	.subagent-cancel:hover {
		border-color: color-mix(in srgb, var(--accent) 28%, var(--border-soft));
		color: var(--text-main);
	}

	.fold-body {
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.68);
		border-radius: 12px;
		padding: 10px 12px;
		color: var(--text-muted);
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.04);
	}

	.subagent-body {
		display: flex;
		flex-direction: column;
		gap: 10px;
		align-self: stretch;
		box-sizing: border-box;
		width: 100%;
		min-width: 0;
	}

	.subagent-purpose {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
	}

	.subagent-progress {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: 100%;
		min-width: 0;
	}

	.subagent-stream,
	.subagent-summary p {
		margin: 0;
		min-width: 0;
		width: 100%;
		max-width: 100%;
		font-size: 11px;
		line-height: 1.5;
		white-space: pre-wrap;
		overflow-wrap: break-word;
		word-break: normal;
		color: var(--text-muted);
	}

	.subagent-thought {
		font-style: italic;
	}

	.subagent-plan {
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	}

	.subagent-tool {
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.55);
		padding: 8px 10px;
	}

	.subagent-tool.pending {
		border-color: rgba(59, 130, 246, 0.18);
	}

	.subagent-tool-header {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 11px;
		color: var(--text-main);
	}

	.subagent-tool-header :global(svg) {
		flex-shrink: 0;
		color: var(--text-muted);
	}

	.subagent-tool-name {
		font-weight: 600;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.subagent-tool-status {
		margin-left: auto;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-muted);
		text-transform: lowercase;
	}

	.subagent-summary {
		box-sizing: border-box;
		width: 100%;
		min-width: 0;
		max-width: 100%;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	.subagent-summary p {
		display: block;
		box-sizing: border-box;
		white-space: normal;
		max-height: 220px;
		overflow-x: hidden;
		overflow-y: auto;
		scrollbar-gutter: stable;
	}
</style>
