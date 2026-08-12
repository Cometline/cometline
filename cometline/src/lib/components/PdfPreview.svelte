<script lang="ts">
	import { Loader } from '@lucide/svelte';
	import { toWikiRelative } from '$lib/wiki/paths';

	let {
		workspacePath,
		filePath,
		wiki,
		reloadVersion = 0
	}: {
		workspacePath: string;
		filePath: string;
		wiki: boolean;
		reloadVersion?: number;
	} = $props();

	let loading = $state(true);
	let error = $state<string | null>(null);
	let previewUrl = $state<string | null>(null);
	let activeToken: string | null = null;
	let loadVersion = 0;

	async function revoke(token: string | null) {
		if (!token) return;
		try {
			await window.electronAPI?.revokePdfPreview?.(token);
		} catch {
			// A revoked or shutting-down main process needs no renderer recovery.
		}
	}

	async function load() {
		const version = ++loadVersion;
		const previousToken = activeToken;
		activeToken = null;
		previewUrl = null;
		loading = true;
		error = null;
		void revoke(previousToken);

		const createPreview = window.electronAPI?.createPdfPreview;
		if (!createPreview) {
			error = 'PDF preview is available in the Cometline desktop app.';
			loading = false;
			return;
		}

		try {
			const result = await createPreview(
				wiki
					? { scope: 'wiki', relativePath: toWikiRelative(filePath) }
					: { scope: 'workspace', workspacePath, relativePath: filePath }
			);
			if (version !== loadVersion) {
				if (result.ok) void revoke(result.token);
				return;
			}
			if (!result.ok) {
				error = result.error;
				return;
			}
			activeToken = result.token;
			previewUrl = result.url;
		} catch (cause) {
			if (version === loadVersion) {
				error = cause instanceof Error ? cause.message : 'Failed to load PDF preview';
			}
		} finally {
			if (version === loadVersion) loading = false;
		}
	}

	$effect(() => {
		void [workspacePath, filePath, wiki, reloadVersion];
		void load();
	});

	$effect(() => () => {
		loadVersion += 1;
		void revoke(activeToken);
	});
</script>

<div class="pdf-preview">
	{#if loading}
		<div class="pdf-preview-state">
			<Loader size={16} stroke-width={2} class="pdf-preview-spinner" />
			<span>Loading PDF…</span>
		</div>
	{:else if error}
		<div class="pdf-preview-state pdf-preview-error">{error}</div>
	{:else if previewUrl}
		<iframe title={filePath} src={previewUrl} class="pdf-preview-frame"></iframe>
	{/if}
</div>

<style>
	.pdf-preview {
		width: 100%;
		height: 100%;
		min-height: 0;
		background: #fff;
	}

	.pdf-preview-frame {
		display: block;
		width: 100%;
		height: 100%;
		border: 0;
	}

	.pdf-preview-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		height: 100%;
		padding: 24px;
		color: #737373;
		font-size: 13px;
		text-align: center;
	}

	.pdf-preview-error {
		color: #b91c1c;
	}

	.pdf-preview-state :global(.pdf-preview-spinner) {
		animation: pdf-preview-spin 0.7s linear infinite;
	}

	@keyframes pdf-preview-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
