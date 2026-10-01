<script lang="ts">
	import { Settings } from '@lucide/svelte';
	import type { SettingsSection } from '../../settings-controller.svelte';
	import type { SettingsPanelMode } from '../../settings-panel-types';

	let {
		mode,
		activeSection,
		onClose
	}: { mode: SettingsPanelMode; activeSection: SettingsSection; onClose: () => void } = $props();
</script>

<header class:window-header={mode === 'window'}>
	<div class="title-mark"><Settings size={16} /></div>
	<div>
		<h2 id="settings-title">Settings</h2>
		<p>
			{#if activeSection === 'models'}
				Enable providers, fetch models, and pick which models power each role.
			{:else if activeSection === 'appearance'}
				Customize hero composer glow, caret trail, and the project icon.
			{:else if activeSection === 'agent'}
				Configure the runtime, OpenCode subagents, skills, and the Discord gateway.
			{:else if activeSection === 'memory'}
				Manage global memories, retrieval thresholds, and compaction.
			{:else if activeSection === 'shortcuts'}
				Customize keyboard shortcuts.
			{:else}
				Startup, storage, updates, and workspace.
			{/if}
		</p>
	</div>
	{#if mode === 'modal'}
		<button class="icon-button" aria-label="Close settings" onclick={onClose}> Close </button>
	{/if}
</header>

<style>
	:global(.modal.window-mode) header {
		min-height: 30px;
		justify-content: center;
		padding: 7px 0 0;
		border-bottom: none;
		text-align: center;
		-webkit-app-region: drag;
	}

	:global(.modal.window-mode) header > * {
		-webkit-app-region: no-drag;
	}

	:global(.modal.window-mode) .title-mark {
		display: none;
	}

	:global(.modal.window-mode) header h2 {
		font-size: 13px;
		font-weight: 700;
		letter-spacing: -0.01em;
		color: color-mix(in srgb, var(--text-main) 74%, transparent);
	}

	:global(.modal.window-mode) header p {
		display: none;
	}

	header {
		display: flex;
		align-items: center;
	}

	header {
		position: sticky;
		top: 0;
		z-index: 2;
		flex-shrink: 0;
		gap: 12px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.96);
	}

	.title-mark {
		width: 32px;
		height: 32px;
		border-radius: 11px;
		background: rgba(0, 102, 204, 0.09);
		color: var(--accent);
		display: grid;
		place-items: center;
	}

	header h2,
	header p {
		margin: 0;
	}

	header h2 {
		font-size: 17px;
		font-weight: 700;
	}

	header p {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	header p {
		min-height: calc(1.45em * 2);
	}

	.icon-button {
		margin-left: auto;
		width: 30px;
		height: 30px;
		border: none;
		border-radius: 9px;
		background: transparent;
		color: var(--text-muted);
		display: grid;
		place-items: center;
		cursor: pointer;
	}

	@media (max-width: 780px) {
		:global(.modal.window-mode) header {
			padding-left: 76px;
			justify-content: flex-start;
			text-align: left;
		}
	}
</style>
