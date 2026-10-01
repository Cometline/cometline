<script lang="ts">
	import { PanelLeftClose, PanelLeftOpen } from '@lucide/svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { miniShellStore } from '$lib/stores/mini-shell.svelte';

	let { sessionId }: { sessionId: string } = $props();

	let openInMainWindowBlocked = $derived(chatStore.isStreamingFor(sessionId));

	async function openInMainWindow() {
		if (!sessionId) return;
		// Guard against the race where the button's disabled state hasn't
		// re-rendered yet but streaming already started/ended: re-check live
		// state at click time rather than trusting only the derived UI flag.
		if (chatStore.isStreamingFor(sessionId)) return;
		await window.electronAPI?.openSessionInMainWindow?.(sessionId);
	}
</script>

<div class="mini-titlebar" aria-label="Mini window drag area">
	<span>Mini Chat</span>
	<Tooltip
		label={miniShellStore.sidebarOpen ? 'Hide chats' : 'Show chats'}
		action="toggleSidebar"
	>
		<button
			class="mini-sidebar-toggle"
			type="button"
			aria-label={miniShellStore.sidebarOpen ? 'Hide chats' : 'Show chats'}
			aria-pressed={miniShellStore.sidebarOpen}
			onclick={() => miniShellStore.toggleSidebar()}
		>
			{#if miniShellStore.sidebarOpen}
				<PanelLeftClose size={15} stroke-width={1.8} />
			{:else}
				<PanelLeftOpen size={15} stroke-width={1.8} />
			{/if}
		</button>
	</Tooltip>
	<button
		class="mini-open-main"
		type="button"
		disabled={openInMainWindowBlocked}
		title={openInMainWindowBlocked
			? 'Wait for the response to finish before opening in the main window'
			: 'Open this chat in the main window'}
		aria-label={openInMainWindowBlocked
			? 'Open this chat in the main window (disabled while responding)'
			: 'Open this chat in the main window'}
		onclick={openInMainWindow}
	>
		<svg viewBox="0 0 16 16" aria-hidden="true">
			<path d="M5 3.5h7.5V11" />
			<path d="M12.5 3.5 6.25 9.75" />
			<path d="M10.5 12.5h-7v-7" />
		</svg>
	</button>
</div>

<style>
	.mini-titlebar {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: var(--mini-titlebar-height);
		z-index: 40;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 0 96px;
		border-bottom: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
		background: color-mix(in srgb, var(--panel-bg) 82%, transparent);
		color: var(--text-muted);
		font-size: 11px;
		font-weight: 650;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		user-select: none;
		-webkit-app-region: drag;
	}

	.mini-open-main {
		position: absolute;
		right: 12px;
		top: 50%;
		transform: translateY(-50%);
		width: 28px;
		height: 28px;
		display: grid;
		place-items: center;
		padding: 0;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 999px;
		background: color-mix(in srgb, var(--panel-bg) 88%, var(--text-main) 6%);
		color: var(--text-main);
		cursor: pointer;
		-webkit-app-region: no-drag;
	}

	.mini-titlebar :global(.tooltip-wrap) {
		position: absolute;
		left: 12px;
		top: 50%;
		transform: translateY(-50%);
		-webkit-app-region: no-drag;
	}

	.mini-sidebar-toggle {
		width: 28px;
		height: 28px;
		display: grid;
		place-items: center;
		padding: 0;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 999px;
		background: color-mix(in srgb, var(--panel-bg) 88%, var(--text-main) 6%);
		color: var(--text-main);
		cursor: pointer;
	}

	.mini-open-main svg {
		width: 14px;
		height: 14px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.7;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.mini-open-main:hover,
	.mini-sidebar-toggle:hover {
		border-color: color-mix(in srgb, var(--hero-composer-glow-color) 54%, var(--border-soft));
		background: color-mix(in srgb, var(--hero-composer-glow-color) 18%, var(--panel-bg));
	}

	.mini-open-main:disabled {
		cursor: not-allowed;
		opacity: 0.4;
	}

	.mini-open-main:disabled:hover {
		border-color: color-mix(in srgb, var(--border-soft) 80%, transparent);
		background: color-mix(in srgb, var(--panel-bg) 88%, var(--text-main) 6%);
	}
</style>
