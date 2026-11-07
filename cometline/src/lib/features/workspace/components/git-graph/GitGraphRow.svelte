<script lang="ts">
	import { GitBranch, Tag } from '@lucide/svelte';
	import {
		laneColor,
		type GitGraphCommit,
		type GitGraphRef
	} from '#lib/features/workspace/git-graph.js';

	let {
		commit,
		columns,
		selected = false,
		headSha = '',
		onOpen,
		onCommitMenu,
		onRefMenu
	}: {
		commit: GitGraphCommit;
		columns: number;
		selected?: boolean;
		headSha?: string;
		onOpen: () => void;
		onCommitMenu: (sha: string, event: MouseEvent) => void;
		onRefMenu: (ref: GitGraphRef, sha: string, event: MouseEvent) => void;
	} = $props();

	const cell = 16;
	const rowHeight = 28;
	const width = $derived(Math.max(columns, 1) * cell + 8);

	function nodeAt(lane: number) {
		return commit.lanes.find((item) => item.lane === lane && item.to == null);
	}
</script>

<div
	class="git-graph-row"
	class:selected
	class:here={commit.sha === headSha}
	role="button"
	tabindex="0"
	onclick={onOpen}
	onkeydown={(event) => {
		if (event.key === 'Enter') onOpen();
	}}
	oncontextmenu={(event) => onCommitMenu(commit.sha, event)}
>
	<svg class="lanes" {width} height={rowHeight} aria-hidden="true">
		{#each Array.from({ length: columns }, (_, lane) => lane) as lane (lane)}
			{@const node = nodeAt(lane)}
			{#if node && (node.kind === 'commit' || node.kind === 'merge')}
				<circle
					cx={lane * cell + cell / 2}
					cy={rowHeight / 2}
					r="3.5"
					fill={laneColor(lane)}
				/>
			{/if}
		{/each}
	</svg>

	<div class="summary">
		{#if commit.refs.length > 0}
			<div class="refs">
				{#each commit.refs as ref (`${ref.kind}:${ref.name}`)}
					<button
						type="button"
						class="ref"
						class:current={ref.current}
						class:tag={ref.kind === 'tag'}
						class:remote={ref.kind === 'remote'}
						class:head={ref.kind === 'head'}
						disabled={ref.kind === 'remote' || ref.kind === 'head'}
						title={ref.name}
						onclick={(event) => onRefMenu(ref, commit.sha, event)}
						oncontextmenu={(event) => onRefMenu(ref, commit.sha, event)}
					>
						{#if ref.kind === 'tag'}
							<Tag size={11} />
						{:else}
							<GitBranch size={11} />
						{/if}
						<span>{ref.name}</span>
					</button>
				{/each}
			</div>
		{/if}
		<span class="subject" title={commit.subject}>{commit.subject}</span>
	</div>

	<button
		type="button"
		class="meta sha"
		title={commit.sha}
		onclick={(event) => onCommitMenu(commit.sha, event)}
	>
		{commit.short_sha}
	</button>
</div>

<style>
	.git-graph-row {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) 64px;
		align-items: center;
		gap: 10px;
		height: 28px;
		width: max-content;
		min-width: 100%;
		padding: 0 10px 0 4px;
	}

	.git-graph-row:hover,
	.git-graph-row.selected {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}

	.git-graph-row.here {
		box-shadow: inset 2px 0 0 var(--accent);
	}

	.lanes {
		display: block;
		overflow: visible;
	}

	.summary {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		overflow: hidden;
	}

	.refs {
		display: flex;
		flex: 0 0 auto;
		flex-wrap: nowrap;
		gap: 4px;
	}

	.ref {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-width: 7.5em;
		max-width: none;
		flex: 0 0 auto;
		border: 1px solid color-mix(in srgb, var(--accent) 28%, var(--border-soft));
		border-radius: 6px;
		padding: 0 6px;
		background: color-mix(in srgb, var(--accent) 14%, var(--panel-bg));
		color: var(--text-main);
		font-size: 11px;
		line-height: 18px;
		cursor: pointer;
	}

	.ref span {
		white-space: nowrap;
	}

	.ref.current {
		background: color-mix(in srgb, var(--accent) 78%, var(--text-main));
		border-color: transparent;
		color: var(--panel-bg);
	}

	.ref.tag {
		background: color-mix(in srgb, var(--status-warning) 16%, var(--panel-bg));
		border-color: color-mix(in srgb, var(--status-warning) 35%, var(--border-soft));
	}

	.ref.remote,
	.ref.head {
		background: color-mix(in srgb, var(--accent) 8%, var(--panel-bg));
		color: var(--text-main);
		cursor: default;
	}

	.ref.head {
		border-color: var(--accent);
		font-weight: 650;
	}

	.subject {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		color: var(--text-main);
	}

	.meta {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12px;
		color: var(--text-muted);
	}

	.sha {
		justify-self: end;
		border: none;
		background: transparent;
		padding: 0;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		cursor: pointer;
	}
</style>
