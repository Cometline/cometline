<script lang="ts">
	import { PanelLeftClose, PanelLeftOpen, PanelRightClose, PanelRightOpen } from '@lucide/svelte';
	import Tooltip from '#lib/components/Tooltip.svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';

	let {
		label,
		renamable,
		titleAttr,
		utilityPage,
		activeSessionId,
		onRename
	}: {
		label: string;
		renamable: boolean;
		titleAttr: string;
		utilityPage: boolean;
		activeSessionId: string | null;
		onRename: () => void;
	} = $props();
</script>

<header class="shell-titlebar" aria-label="Window title bar">
	<div class="shell-titlebar-start">
		<Tooltip
			label={shellStore.sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
			action="toggleSidebar"
		>
			<button
				type="button"
				class="shell-titlebar-btn"
				aria-label={shellStore.sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
				aria-pressed={shellStore.sidebarOpen}
				onclick={() => shellStore.toggleSidebar()}
			>
				{#if shellStore.sidebarOpen}
					<PanelLeftClose size={16} stroke-width={1.8} />
				{:else}
					<PanelLeftOpen size={16} stroke-width={1.8} />
				{/if}
			</button>
		</Tooltip>
	</div>
	{#if label}
		{#if renamable}
			<button
				type="button"
				class="shell-titlebar-title"
				title={titleAttr}
				aria-label={`Rename session: ${label}`}
				ondblclick={onRename}
			>
				{label}
			</button>
		{:else}
			<span class="shell-titlebar-title">{label}</span>
		{/if}
	{/if}
	{#if !utilityPage}
		<div class="shell-titlebar-end">
			<Tooltip
				label={shellStore.workspacePanelOpen
					? 'Hide workspace panel'
					: 'Show workspace panel'}
				action="toggleWorkspacePanel"
			>
				<button
					type="button"
					class="shell-titlebar-btn"
					aria-label={shellStore.workspacePanelOpen
						? 'Hide workspace panel'
						: 'Show workspace panel'}
					aria-pressed={shellStore.workspacePanelOpen}
					disabled={!activeSessionId}
					onclick={() => shellStore.toggleWorkspacePanel()}
				>
					{#if shellStore.workspacePanelOpen}
						<PanelRightClose size={16} stroke-width={1.8} />
					{:else}
						<PanelRightOpen size={16} stroke-width={1.8} />
					{/if}
				</button>
			</Tooltip>
		</div>
	{/if}
</header>

<style>
	.shell-titlebar {
		position: relative;
		flex-shrink: 0;
		height: var(--panel-header-height);
		z-index: 40;
		display: flex;
		align-items: center;
		box-sizing: border-box;
		padding: 0 10px;
		border-bottom: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		background: transparent;
		-webkit-app-region: drag;
	}

	.shell-titlebar-start,
	.shell-titlebar-end {
		position: relative;
		z-index: 1;
		display: flex;
		align-items: center;
		-webkit-app-region: no-drag;
	}

	.shell-titlebar-end {
		margin-left: auto;
	}

	.shell-titlebar-title {
		position: absolute;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
		min-width: 0;
		max-width: min(calc(100% - 88px), 14rem);
		margin: 0;
		padding: 0;
		border: none;
		background: transparent;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-muted);
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.01em;
		line-height: 1.25;
		user-select: none;
		text-align: center;
		cursor: default;
		-webkit-app-region: no-drag;
	}

	@media (min-width: 900px) {
		.shell-titlebar-title {
			max-width: min(calc(100% - 88px), 22rem);
		}
	}

	@media (min-width: 1280px) {
		.shell-titlebar-title {
			max-width: min(calc(100% - 88px), 32rem);
		}
	}

	.shell-titlebar-btn {
		width: 26px;
		height: 26px;
		padding: 0;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		display: grid;
		place-items: center;
		cursor: pointer;
		flex-shrink: 0;
		-webkit-app-region: no-drag;
	}

	.shell-titlebar-btn:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.shell-titlebar-btn:active:not(:disabled) {
		background: rgba(0, 0, 0, 0.07);
	}

	.shell-titlebar-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}
</style>
