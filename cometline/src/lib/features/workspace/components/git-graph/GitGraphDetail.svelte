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
			<p class="field">
				<strong>Commit:</strong>
				<span class="mono hash" title={detail.sha}>{detail.sha}</span>
			</p>
			<p class="field">
				<strong>Parents:</strong>
				<span class="parents">
					{#each detail.parents as parent, index (parent)}
						<span class="mono link" title={parent}>{parent.slice(0, 12)}</span>{index <
						detail.parents.length - 1
							? ', '
							: ''}
					{/each}
				</span>
			</p>
			<p class="field">
				<strong>Author:</strong>
				<span class="author-line">
					{detail.author}
					<span class="mono" title={detail.author_email}
						>&lt;{detail.author_email}&gt;</span
					>
				</span>
			</p>
			<p class="field"><strong>Date:</strong> <span>{when}</span></p>
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
					{#if fileDir(file.path)}
						<span class="dir" title={file.path}>{fileDir(file.path)}</span>
					{/if}
					<span class="file-main">
						<span class="name">
							<FileTypeIcon path={file.path} size={14} />
							<span class="filename" title={fileName(file.path)}
								>{fileName(file.path)}</span
							>
						</span>
						{#if file.status !== 'binary'}
							<span class="stat" aria-label="+{file.additions} -{file.deletions}">
								<span class="add">+{file.additions}</span>
								<span class="del">-{file.deletions}</span>
							</span>
						{/if}
					</span>
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
		isolation: isolate;
		display: grid;
		grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr) 24px;
		grid-template-areas: 'info files close';
		gap: 12px;
		margin: 0 8px 8px;
		padding: 10px 8px 10px 12px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
		border-radius: 8px;
		background: color-mix(in srgb, var(--text-main) 3%, var(--panel-bg));
	}

	.close {
		grid-area: close;
		align-self: start;
		justify-self: end;
		display: inline-flex;
		border: none;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.info {
		grid-area: info;
		min-width: 0;
		overflow: hidden;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
	}

	.files {
		grid-area: files;
		min-width: 0;
		overflow: hidden;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
	}

	.field,
	.muted {
		margin: 0 0 4px;
	}

	.field {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 4px 6px;
		align-items: start;
	}

	.field > strong {
		white-space: nowrap;
	}

	.parents,
	.author-line {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.mono {
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 11px;
	}

	.hash {
		min-width: 0;
		overflow-wrap: anywhere;
		word-break: break-all;
	}

	.link {
		color: var(--accent);
	}

	.body {
		margin-top: 8px;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.file {
		display: flex;
		flex-direction: column;
		gap: 2px;
		width: 100%;
		max-width: 100%;
		border: none;
		background: transparent;
		padding: 3px 0;
		text-align: left;
		color: inherit;
		cursor: pointer;
	}

	.dir {
		color: var(--text-muted);
		font-size: 11px;
		line-height: 1.35;
		overflow-wrap: anywhere;
		word-break: break-word;
	}

	.file-main {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		width: 100%;
	}

	.name {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
	}

	.filename {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.stat {
		flex: 0 0 auto;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 11px;
		white-space: nowrap;
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
			grid-template-columns: minmax(0, 1fr) 24px;
			grid-template-areas:
				'info close'
				'files files';
		}
	}
</style>
