<script lang="ts">
	import type { Snippet } from 'svelte';
	import { ChevronDown, ChevronRight, Minus, Plus, RotateCcw } from '@lucide/svelte';
	import type {
		GitChangesSectionKind,
		GitFile
	} from '#lib/features/workspace/git-changes-browser.js';

	let {
		section,
		files,
		open = $bindable(),
		mutating,
		onUnstageAll,
		onDiscardAll,
		onStageAll,
		row
	}: {
		section: GitChangesSectionKind;
		files: GitFile[];
		open: boolean;
		mutating: boolean;
		onUnstageAll?: () => unknown;
		onDiscardAll?: () => void;
		onStageAll?: () => unknown;
		row: Snippet<[GitFile, GitChangesSectionKind]>;
	} = $props();
</script>

<section class="git-section" class:git-section-staged={section === 'staged'}>
	<div class="git-section-header">
		<button
			type="button"
			class="git-section-toggle"
			onclick={() => (open = !open)}
			aria-expanded={open}
		>
			<span class="git-section-chevron">
				{#if open}
					<ChevronDown size={13} stroke-width={2} />
				{:else}
					<ChevronRight size={13} stroke-width={2} />
				{/if}
			</span>
			<span class="git-section-title"
				>{section === 'staged' ? 'Staged Changes' : 'Changes'}</span
			>
			<span class="git-section-count">{files.length}</span>
		</button>
		{#if files.length > 0}
			{#if section === 'staged'}
				<button
					type="button"
					class="git-section-action"
					title="Unstage all"
					disabled={mutating}
					onclick={() => void onUnstageAll?.()}
				>
					<Minus size={13} />
				</button>
			{:else}
				<button
					type="button"
					class="git-section-action danger"
					title="Discard all"
					disabled={mutating}
					onclick={() => onDiscardAll?.()}
				>
					<RotateCcw size={13} />
				</button>
				<button
					type="button"
					class="git-section-action"
					title="Stage all"
					disabled={mutating}
					onclick={() => void onStageAll?.()}
				>
					<Plus size={13} />
				</button>
			{/if}
		{/if}
	</div>
	{#if open && files.length > 0}
		<ul class="git-file-list">
			{#each files as file (file.path + ':' + section)}
				{@render row(file, section)}
			{/each}
		</ul>
	{/if}
</section>

<style>
	/* Mirrors sidebar WorkspaceGroup: soft pill group + nested session-like rows. */
	.git-section {
		display: flex;
		flex-direction: column;
		gap: 4px;
		border-radius: 8px;
		padding: 2px;
		border: 1px solid
			color-mix(
				in srgb,
				var(--workspace-inactive-color, var(--workspace-group-color)) 14%,
				transparent
			);
		background: linear-gradient(
			135deg,
			color-mix(
				in srgb,
				var(--workspace-inactive-color, var(--workspace-group-color)) 16%,
				transparent
			),
			color-mix(
				in srgb,
				var(--workspace-inactive-color, var(--workspace-group-color)) 6%,
				transparent
			)
		);
		transition:
			background var(--duration-fast, 150ms) var(--ease-smooth, ease),
			border-color var(--duration-fast, 150ms) var(--ease-smooth, ease),
			box-shadow var(--duration-fast, 150ms) var(--ease-smooth, ease);
	}

	/* Staged = “active” workspace group: hero glow surface like the focused sidebar group. */
	.git-section-staged {
		--git-section-accent: var(--hero-composer-glow-color, var(--accent));
		/* Recompute session-row tokens so nested file hovers pick up the active tint. */
		--session-row-bg-hover: color-mix(in srgb, var(--git-section-accent) 11%, transparent);
		--session-row-ring: color-mix(in srgb, var(--git-section-accent) 36%, var(--border-soft));
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--git-section-accent) 31%, transparent),
			color-mix(in srgb, var(--git-section-accent) 12%, transparent)
		);
		border-color: color-mix(in srgb, var(--git-section-accent) 26%, transparent);
		box-shadow: 0 8px 22px color-mix(in srgb, var(--git-section-accent) 8%, transparent);
	}

	.git-section-staged:hover {
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--git-section-accent) 38%, transparent),
			color-mix(in srgb, var(--git-section-accent) 16%, transparent)
		);
	}

	.git-section-header {
		display: flex;
		align-items: center;
		gap: 2px;
		padding: 0 2px 0 0;
	}

	.git-section-toggle {
		display: flex;
		align-items: center;
		gap: 6px;
		flex: 1;
		min-width: 0;
		border: none;
		border-radius: 7px;
		padding: 6px 8px;
		background: transparent;
		color: var(--workspace-inactive-color, var(--workspace-group-color));
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		text-align: left;
		cursor: pointer;
	}

	.git-section:hover .git-section-toggle {
		color: var(--text-muted);
	}

	.git-section-staged .git-section-toggle {
		color: var(--text-main);
	}

	.git-section-staged .git-section-chevron {
		color: var(--git-section-accent);
	}

	.git-section-chevron {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: inherit;
	}

	.git-section-title {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.git-section-count {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.git-section-staged .git-section-count {
		color: color-mix(in srgb, var(--git-section-accent) 55%, var(--text-main));
		background: color-mix(in srgb, var(--git-section-accent) 18%, transparent);
	}

	.git-section-action {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		margin-right: 2px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.git-section-action:hover:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, var(--workspace-group-color)) 18%,
			transparent
		);
		color: var(--text-primary, var(--color-111111));
	}

	.git-section-action.danger:hover:not(:disabled) {
		background: rgba(185, 28, 28, 0.1);
		color: var(--color-b91c1c);
	}

	.git-section-staged .git-section-action:hover:not(:disabled) {
		background: color-mix(in srgb, var(--git-section-accent) 20%, transparent);
		color: var(--git-section-accent);
	}

	.git-section-action:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	/* Nested under group header — same visual tab as sidebar sessions under a workspace. */
	.git-file-list {
		list-style: none;
		margin: 0;
		padding: 0 2px 2px;
		display: flex;
		flex-direction: column;
		gap: 2px;
		/* Small indent so files sit under the group, like SessionRow under WorkspaceGroup. */
		padding-left: 10px;
	}
</style>
