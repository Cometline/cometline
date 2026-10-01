<script lang="ts">
	import { Copy, MessageSquarePlus, Minus, Plus, RotateCcw } from '@lucide/svelte';

	let {
		filePath,
		mutating,
		canStage,
		canUnstage,
		canDiscard,
		copyFlash,
		hasDiff,
		actionError,
		onBack,
		onStage,
		onUnstage,
		onDiscard,
		onCopyPath,
		onAddPath,
		onAddDiff
	}: {
		filePath: string;
		mutating: boolean;
		canStage: boolean;
		canUnstage: boolean;
		canDiscard: boolean;
		copyFlash: boolean;
		hasDiff: boolean;
		actionError: string | null;
		onBack?: () => void;
		onStage: () => unknown;
		onUnstage: () => unknown;
		onDiscard: () => void;
		onCopyPath: () => unknown;
		onAddPath: () => void;
		onAddDiff: () => void;
	} = $props();
</script>

<header class="git-diff-header">
	<div class="git-diff-title-row">
		{#if onBack}
			<button type="button" class="git-diff-back" onclick={onBack}>← Files</button>
		{/if}
		<span class="git-diff-path" title={filePath}>{filePath}</span>
	</div>
	<div class="git-diff-actions">
		<button
			type="button"
			class="git-diff-action"
			disabled={mutating || !canStage}
			onclick={() => void onStage()}
			title={canStage ? 'Stage' : 'Already staged'}
		>
			<Plus size={14} />
			<span>Stage</span>
		</button>
		<button
			type="button"
			class="git-diff-action"
			disabled={mutating || !canUnstage}
			onclick={() => void onUnstage()}
			title={canUnstage ? 'Unstage' : 'Nothing staged'}
		>
			<Minus size={14} />
			<span>Unstage</span>
		</button>
		<button
			type="button"
			class="git-diff-action danger"
			disabled={mutating || !canDiscard}
			onclick={onDiscard}
			title={canDiscard ? 'Discard' : 'No working-tree changes to discard'}
		>
			<RotateCcw size={14} />
			<span>Discard</span>
		</button>
		<button
			type="button"
			class="git-diff-action"
			onclick={() => void onCopyPath()}
			title="Copy path"
		>
			<Copy size={14} />
			<span>{copyFlash ? 'Copied' : 'Copy path'}</span>
		</button>
		<button type="button" class="git-diff-action" onclick={onAddPath} title="Add path to chat">
			<MessageSquarePlus size={14} />
			<span>Add path</span>
		</button>
		{#if hasDiff}
			<button
				type="button"
				class="git-diff-action"
				onclick={onAddDiff}
				title="Add full file diff to chat"
			>
				<span>Add diff</span>
			</button>
		{/if}
	</div>
	{#if actionError}
		<p class="git-diff-action-error" role="alert">{actionError}</p>
	{/if}
</header>

<style>
	.git-diff-header {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px 12px;
		border-bottom: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
	}

	.git-diff-title-row {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.git-diff-back {
		flex: 0 0 auto;
		border: none;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		padding: 2px 0;
	}

	.git-diff-back:hover {
		color: var(--text-primary, var(--color-111111));
	}

	.git-diff-path {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		font-weight: 600;
		color: var(--text-primary, var(--color-111111));
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}

	.git-diff-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.git-diff-action {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		border: 1px solid var(--border-soft, rgba(0, 0, 0, 0.1));
		border-radius: 7px;
		padding: 4px 8px;
		background: var(--surface-elevated, var(--panel-bg));
		color: var(--text-primary, var(--color-111111));
		font-size: 12px;
		font-weight: 550;
		cursor: pointer;
	}

	.git-diff-action:hover:not(:disabled) {
		border-color: var(--text-soft, rgba(0, 0, 0, 0.2));
	}

	.git-diff-action:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.git-diff-action.danger:hover:not(:disabled) {
		border-color: rgba(185, 28, 28, 0.35);
		color: var(--color-b91c1c);
	}

	.git-diff-action-error {
		margin: 0;
		font-size: 12px;
		color: var(--status-error, var(--color-b91c1c));
	}
</style>
