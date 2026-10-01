<script lang="ts">
	import { Brain, Keyboard, Palette, Power, Settings, Workflow } from '@lucide/svelte';
	import type { SettingsSection } from '../../settings-controller.svelte';

	let {
		activeSection,
		navSectionDirty,
		onSelect
	}: {
		activeSection: SettingsSection;
		navSectionDirty: (section: SettingsSection) => boolean;
		onSelect: (section: SettingsSection) => void;
	} = $props();

	const items = [
		{ section: 'models', label: 'Models', icon: Settings, tracksPending: true },
		{ section: 'memory', label: 'Memory', icon: Brain, tracksPending: true },
		{ section: 'agent', label: 'Agent', icon: Workflow, tracksPending: true },
		{ section: 'appearance', label: 'Appearance', icon: Palette, tracksPending: true },
		{ section: 'shortcuts', label: 'Shortcuts', icon: Keyboard, tracksPending: false },
		{ section: 'app', label: 'App', icon: Power, tracksPending: true }
	] as const;
</script>

<nav class="settings-nav" aria-label="Settings sections">
	{#each items as item (item.section)}
		<button
			class="settings-nav-item"
			class:selected={activeSection === item.section}
			class:has-pending={item.tracksPending && navSectionDirty(item.section)}
			onclick={() => onSelect(item.section)}
		>
			<item.icon size={15} />
			<span>{item.label}</span>
		</button>
	{/each}
</nav>

<style>
	:global(.modal.window-mode) .settings-nav {
		display: flex;
		justify-content: center;
		gap: 18px;
		padding: 0 72px 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		-webkit-app-region: drag;
	}

	:global(.modal.window-mode) .settings-nav-item {
		position: relative;
		display: grid;
		place-items: center;
		gap: 3px;
		min-width: 72px;
		border: none;
		border-radius: 12px;
		background: transparent;
		padding: 7px 8px 8px;
		font-size: 11px;
		font-weight: 700;
		color: color-mix(in srgb, var(--text-main) 58%, transparent);
		box-shadow: none;
		-webkit-app-region: no-drag;
	}

	:global(.modal.window-mode) .settings-nav-item :global(svg) {
		width: 20px;
		height: 20px;
		stroke-width: 1.9;
	}

	:global(.modal.window-mode) .settings-nav-item.selected {
		background: color-mix(in srgb, var(--text-main) 7%, transparent);
		color: var(--text-main);
		box-shadow: none;
		border: 1px solid var(--border-soft);
	}

	:global(.modal.window-mode) .settings-nav-item.has-pending::after {
		content: '';
		position: absolute;
		top: 9px;
		right: 12px;
		width: 5px;
		height: 5px;
		border-radius: 999px;
		background: var(--accent);
	}

	.settings-nav {
		display: grid;
		gap: 8px;
		align-content: start;
	}

	.settings-nav-item {
		display: flex;
		align-items: center;
		gap: 8px;
		/* width: 70%; */
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.72);
		padding: 10px 12px;
		font: inherit;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
		text-align: left;
		cursor: pointer;
	}

	.settings-nav-item.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	@media (max-width: 780px) {
		:global(.modal.window-mode) .settings-nav {
			justify-content: flex-start;
			gap: 8px;
			overflow-x: auto;
			padding: 0 0 10px;
		}

		:global(.modal.window-mode) .settings-nav-item {
			min-width: 68px;
		}
	}
</style>
