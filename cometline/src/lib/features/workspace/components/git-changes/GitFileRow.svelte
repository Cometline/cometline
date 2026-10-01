<script lang="ts">
	import { Copy, MessageSquarePlus, Minus, Plus, RotateCcw } from '@lucide/svelte';
	import FileTypeIcon from '$lib/features/workspace/components/FileTypeIcon.svelte';
	import {
		gitFileDir,
		gitFileName,
		gitStatusBadge,
		type GitChangesSectionKind,
		type GitFile
	} from '$lib/features/workspace/git-changes-browser';

	let {
		file,
		section,
		mutating,
		copied,
		onOpen,
		onStage,
		onUnstage,
		onDiscard,
		onCopy,
		onAddToChat
	}: {
		file: GitFile;
		section: GitChangesSectionKind;
		mutating: boolean;
		copied: boolean;
		onOpen: (path: string) => void;
		onStage: (path: string) => unknown;
		onUnstage: (path: string) => unknown;
		onDiscard: (path: string) => void;
		onCopy: (path: string) => unknown;
		onAddToChat: (path: string) => void;
	} = $props();
</script>

<li class="git-file-row" title={file.path}>
	<button type="button" class="git-file-main" onclick={() => onOpen(file.path)}>
		<span class="git-file-icon" aria-hidden="true">
			<FileTypeIcon path={file.path} size={16} />
		</span>
		<span class="git-file-labels">
			<span class="git-file-name">{gitFileName(file.path)}</span>
			{#if gitFileDir(file.path)}
				<span class="git-file-dir">{gitFileDir(file.path)}</span>
			{/if}
		</span>
	</button>
	<span class="git-row-actions">
		{#if section === 'changes'}
			<button
				type="button"
				class="git-icon-btn"
				title="Stage"
				aria-label="Stage"
				disabled={mutating}
				onclick={() => void onStage(file.path)}
			>
				<Plus size={13} />
			</button>
			<button
				type="button"
				class="git-icon-btn danger"
				title="Discard"
				aria-label="Discard"
				disabled={mutating}
				onclick={() => onDiscard(file.path)}
			>
				<RotateCcw size={13} />
			</button>
		{:else}
			<button
				type="button"
				class="git-icon-btn"
				title="Unstage"
				aria-label="Unstage"
				disabled={mutating}
				onclick={() => void onUnstage(file.path)}
			>
				<Minus size={13} />
			</button>
		{/if}
		<button
			type="button"
			class="git-icon-btn"
			title={copied ? 'Copied' : 'Copy path'}
			aria-label="Copy path"
			onclick={() => void onCopy(file.path)}
		>
			<Copy size={13} />
		</button>
		<button
			type="button"
			class="git-icon-btn"
			title="Add path to chat"
			aria-label="Add path to chat"
			onclick={() => onAddToChat(file.path)}
		>
			<MessageSquarePlus size={13} />
		</button>
	</span>
	<span
		class="git-badge"
		class:untracked={file.untracked}
		class:deleted={file.status === 'deleted'}
		class:added={file.status === 'added' || file.untracked}
		aria-label={file.status}
	>
		{gitStatusBadge(file)}
	</span>
</li>

<style>
	.git-file-row {
		display: flex;
		align-items: center;
		gap: 2px;
		width: 100%;
		border-radius: 8px;
		padding: 0 4px 0 0;
		color: var(--text-primary, #111);
		font-size: 13px;
		transition:
			background-color var(--duration-fast, 150ms) var(--ease-smooth, ease),
			box-shadow var(--duration-fast, 150ms) var(--ease-smooth, ease);
	}

	.git-file-row:hover {
		background: var(--session-row-bg-hover, rgba(0, 0, 0, 0.04));
		box-shadow: inset 0 0 0 1px var(--session-row-ring, rgba(0, 0, 0, 0.06));
	}

	.git-file-row:hover .git-row-actions {
		opacity: 1;
	}

	.git-file-main {
		display: flex;
		align-items: center;
		gap: 8px;
		flex: 1;
		min-width: 0;
		border: none;
		border-radius: 8px;
		padding: 6px 6px 6px 8px;
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.git-file-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 16px;
		width: 16px;
		height: 16px;
	}

	.git-file-labels {
		display: flex;
		align-items: baseline;
		gap: 6px;
		flex: 1;
		min-width: 0;
		overflow: hidden;
	}

	.git-file-name {
		flex: 0 1 auto;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		color: var(--text-primary, #111);
	}

	.git-file-dir {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 11px;
		color: var(--text-muted);
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}

	/* Rightmost column — after action icons. */
	.git-badge {
		flex: 0 0 auto;
		min-width: 18px;
		margin-left: 2px;
		padding: 0 2px;
		text-align: center;
		font-size: 11px;
		font-weight: 700;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		color: #b45309;
	}

	.git-badge.untracked,
	.git-badge.added {
		color: #15803d;
	}

	.git-badge.deleted {
		color: #b91c1c;
	}

	.git-row-actions {
		display: inline-flex;
		gap: 1px;
		opacity: 0;
		flex: 0 0 auto;
	}

	.git-icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 5px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.git-icon-btn:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.06);
		color: var(--text-primary, #111);
	}

	.git-icon-btn.danger:hover:not(:disabled) {
		color: #b91c1c;
		background: rgba(239, 68, 68, 0.1);
	}

	.git-icon-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
</style>
