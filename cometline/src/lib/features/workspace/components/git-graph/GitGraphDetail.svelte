<script lang="ts">
	import { X } from '@lucide/svelte';
	import FileTypeIcon from '#lib/features/workspace/components/FileTypeIcon.svelte';
	import {
		fileDir,
		fileName,
		formatGraphDate,
		type GitGraphCommitDetail
	} from '#lib/features/workspace/git-graph.js';

	let {
		detail,
		loading,
		error,
		onClose,
		onOpenFile
	}: {
		detail: GitGraphCommitDetail | null;
		loading: boolean;
		error: string | null;
		onClose: () => void;
		onOpenFile?: (path: string) => void;
	} = $props();

	const when = $derived(
		detail?.authored_at
			? new Date(detail.authored_at * 1000).toLocaleString(undefined, {
					weekday: 'short',
					year: 'numeric',
					month: 'short',
					day: 'numeric',
					hour: '2-digit',
					minute: '2-digit',
					second: '2-digit',
					timeZoneName: 'long'
				})
			: formatGraphDate(detail?.authored_at)
	);
</script>

<div class="detail">
	<button type="button" class="close" aria-label="Close commit info" onclick={onClose}>
		<X size={14} />
	</button>
	{#if loading && !detail}
		<p class="muted">Loading commit…</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else if detail}
		<div class="info">
			<p><strong>Commit:</strong> <span class="mono">{detail.sha}</span></p>
			<p>
				<strong>Parents:</strong>
				{#each detail.parents as parent, index (parent)}
					<span class="mono link">{parent.slice(0, 12)}</span>{index <
					detail.parents.length - 1
						? ', '
						: ''}
				{/each}
			</p>
			<p>
				<strong>Author:</strong>
				{detail.author} <span class="mono">&lt;{detail.author_email}&gt;</span>
			</p>
			<p><strong>Date:</strong> {when}</p>
			{#if detail.body}
				<p class="body">{detail.body}</p>
			{:else}
				<p class="body">{detail.subject}</p>
			{/if}
		</div>
		<div class="files">
			{#each detail.files as file (file.path)}
				<button
					type="button"
					class="file"
					title={file.path}
					onclick={() => onOpenFile?.(file.path)}
				>
					<span class="dir">{fileDir(file.path)}</span>
					<span class="name">
						<FileTypeIcon path={file.path} size={14} />
						{fileName(file.path)}
					</span>
					{#if file.status !== 'binary'}
						<span class="stat">
							<span class="add">+{file.additions}</span>
							<span class="del">-{file.deletions}</span>
						</span>
					{/if}
				</button>
			{:else}
				<p class="muted">No file changes.</p>
			{/each}
		</div>
	{/if}
</div>

<style>
	.detail {
		position: relative;
		z-index: 1;
		display: grid;
		grid-template-columns: minmax(0, 1.1fr) minmax(0, 0.9fr) 28px;
		gap: 12px;
		margin: 0 8px 8px;
		padding: 10px 8px 10px 12px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
		border-radius: 8px;
		background: color-mix(in srgb, var(--text-main) 3%, var(--panel-bg));
	}

	.close {
		grid-column: 3;
		grid-row: 1;
		align-self: start;
		justify-self: end;
		display: inline-flex;
		border: none;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.info,
	.files {
		min-width: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
	}

	.info p,
	.muted {
		margin: 0 0 4px;
	}

	.mono {
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 11px;
	}

	.link {
		color: var(--accent);
	}

	.body {
		margin-top: 8px;
		white-space: pre-wrap;
	}

	.file {
		display: grid;
		width: 100%;
		border: none;
		background: transparent;
		text-align: left;
		color: inherit;
		cursor: pointer;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 2px 8px;
		padding: 3px 0;
	}

	.dir {
		grid-column: 1 / -1;
		color: var(--text-muted);
		font-size: 11px;
	}

	.name {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.stat {
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 11px;
	}

	.add {
		color: var(--status-success);
	}

	.del {
		color: var(--status-error);
	}

	.error {
		color: var(--status-error);
	}

	@media (max-width: 720px) {
		.detail {
			grid-template-columns: minmax(0, 1fr) 28px;
		}

		.files {
			grid-column: 1;
		}
	}
</style>
