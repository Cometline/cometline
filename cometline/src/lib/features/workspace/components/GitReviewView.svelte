<script lang="ts">
	import GitReviewFileBlock from '#lib/features/workspace/components/git-changes/GitReviewFileBlock.svelte';
	import type { GitReviewFile } from '#lib/features/workspace/git-changes-browser.js';

	let {
		workspacePath,
		files,
		onBack,
		onMutated
	}: {
		workspacePath: string;
		files: GitReviewFile[];
		onBack: () => void;
		onMutated: () => unknown;
	} = $props();

	/** How many file diffs to fetch around the viewport. */
	const WINDOW = 6;

	let scroller = $state<HTMLDivElement | null>(null);
	let start = $state(0);

	const visibleEnd = $derived(Math.min(files.length, start + WINDOW));

	function updateWindow() {
		const root = scroller;
		if (!root) return;
		const blocks = root.querySelectorAll<HTMLElement>('[data-review-index]');
		const top = root.getBoundingClientRect().top;
		let first = 0;
		for (const block of blocks) {
			const index = Number(block.dataset.reviewIndex);
			if (block.getBoundingClientRect().bottom >= top - 40) {
				first = Number.isFinite(index) ? index : 0;
				break;
			}
		}
		start = Math.max(0, first - 1);
	}

	function onScroll() {
		updateWindow();
	}
</script>

<div class="git-review">
	<div class="git-review-bar">
		<button type="button" class="git-review-back" onclick={onBack}>← Files</button>
		<span class="git-review-count">
			{files.length} file{files.length === 1 ? '' : 's'}
		</span>
	</div>
	<div class="git-review-scroll scrollbar-none" bind:this={scroller} onscroll={onScroll}>
		{#each files as file, index (`${file.section}:${file.path}`)}
			<div data-review-index={index}>
				<GitReviewFileBlock
					{workspacePath}
					{file}
					active={index >= start && index < visibleEnd}
					{onMutated}
				/>
			</div>
		{/each}
	</div>
</div>

<style>
	.git-review {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
	}

	.git-review-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 8px 12px;
		border-bottom: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
	}

	.git-review-back {
		border: none;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		padding: 2px 0;
	}

	.git-review-back:hover {
		color: var(--text-primary, var(--color-111111));
	}

	.git-review-count {
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.git-review-scroll {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 6px 8px 12px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
</style>
