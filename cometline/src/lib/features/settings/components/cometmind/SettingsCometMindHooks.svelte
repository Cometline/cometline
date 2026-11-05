<script lang="ts">
	import { onMount } from 'svelte';
	import { formatIdList, parseIdList, type CometMindSettings } from '#lib/cometmind-settings.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { listHooksCatalog } from '#lib/generated/cometmind-api/index.js';

	let { cometmind = $bindable() }: { cometmind: CometMindSettings } = $props();

	let trustedPathsText = $state(formatIdList(cometmind.hooks.trustedWorkspacePaths));
	let catalogEntries = $state<
		Array<{ event: string; matcher: string; command: string; source: string }>
	>([]);
	let catalogEnabled = $state(false);
	let projectHooksAllowed = $state(false);
	let catalogError = $state('');

	onMount(() => {
		void refreshCatalog();
	});

	async function refreshCatalog() {
		catalogError = '';
		try {
			const workspacePath = shellStore.workspacePath ?? '';
			const res = await listHooksCatalog({
				query: workspacePath ? { workspace_path: workspacePath } : undefined
			});
			if (res.error || !res.data) {
				catalogError = 'Sidecar catalog unavailable';
				catalogEntries = [];
				return;
			}
			catalogEnabled = res.data.enabled;
			projectHooksAllowed = res.data.project_hooks_allowed;
			catalogEntries = res.data.entries ?? [];
		} catch {
			catalogError = 'Sidecar catalog unavailable';
		}
	}

	function syncTrustedPaths() {
		cometmind = {
			...cometmind,
			hooks: {
				...cometmind.hooks,
				trustedWorkspacePaths: parseIdList(trustedPathsText)
			}
		};
	}

	function trustCurrentWorkspace() {
		const path = shellStore.workspacePath?.trim();
		if (!path) return;
		const paths = new Set(parseIdList(trustedPathsText));
		paths.add(path);
		trustedPathsText = formatIdList([...paths]);
		syncTrustedPaths();
	}
</script>

<div class="settings-subsection">
	<h3>Tool hooks</h3>
	<p class="settings-hint">
		Subprocess PreToolUse / PostToolUse hooks around built-in tools. Disabled by default. Config:
		<code>~/.cometmind/hooks.json</code>, project <code>.cometmind/hooks.json</code> (requires trust
		below), and <code>plugins/*/plugin.json</code>.
	</p>
	<label class="settings-checkbox">
		<input type="checkbox" bind:checked={cometmind.hooks.enabled} />
		Enable tool hooks
	</label>
	<label>
		<span>Trusted workspace paths (project hooks)</span>
		<textarea
			rows="3"
			bind:value={trustedPathsText}
			oninput={syncTrustedPaths}
			placeholder="/path/to/your/project"
		></textarea>
	</label>
	<button type="button" class="settings-inline-button" onclick={trustCurrentWorkspace}>
		Trust current workspace
	</button>
	{#if catalogError}
		<p class="settings-hint">{catalogError}</p>
	{:else}
		<p class="settings-hint">
			Catalog: {catalogEnabled ? 'enabled' : 'disabled'} · project hooks
			{projectHooksAllowed ? 'allowed' : 'blocked'} for current workspace · {catalogEntries.length}
			handler(s)
		</p>
		{#if catalogEntries.length > 0}
			<ul class="hooks-catalog">
				{#each catalogEntries as entry (entry.source + entry.event + entry.matcher)}
					<li>
						<strong>{entry.event}</strong>
						{#if entry.matcher}
							<code>{entry.matcher}</code>
						{/if}
						→ <code>{entry.command}</code>
						<span class="hooks-source">{entry.source}</span>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
</div>

<style>
	.hooks-catalog {
		margin: 0.5rem 0 0;
		padding-left: 1.2rem;
		font-size: 0.85rem;
	}
	.hooks-source {
		display: block;
		opacity: 0.7;
		font-size: 0.8rem;
	}
</style>
