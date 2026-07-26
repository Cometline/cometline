<script lang="ts">
	import { tick } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { fly, fade } from 'svelte/transition';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';
	import { miniShellStore } from '$lib/stores/mini-shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { createMiniWindowSession, navigateMiniToSession } from '$lib/mini-window-session';
	import MiniSessionSidebar from '$lib/components/MiniSessionSidebar.svelte';
	import type { Session } from '$lib/types';

	let { children } = $props();
	let creatingSession = $state(false);
	let sidebarRef = $state<{ focusSearch: () => void } | null>(null);

	async function createSession() {
		if (creatingSession) return;
		creatingSession = true;
		try {
			await createMiniWindowSession();
			miniShellStore.closeSidebar();
		} finally {
			creatingSession = false;
		}
	}

	async function selectSession(session: Session) {
		await navigateMiniToSession(session);
		miniShellStore.closeSidebar();
	}

	async function focusSearch() {
		miniShellStore.openSidebar();
		await tick();
		sidebarRef?.focusSearch();
	}

	function onKeydown(event: KeyboardEvent) {
		if (matchesShortcut(event, settingsStore.settings.shortcuts.toggleSidebar)) {
			event.preventDefault();
			miniShellStore.toggleSidebar();
			return;
		}
		if (matchesShortcut(event, settingsStore.settings.shortcuts.focusSearch)) {
			event.preventDefault();
			void focusSearch();
			return;
		}
		if (matchesShortcut(event, settingsStore.settings.shortcuts.findInSession)) {
			event.preventDefault();
			miniShellStore.closeSidebar();
			shellStore.requestSessionFind();
			return;
		}
		if (matchesShortcut(event, settingsStore.settings.shortcuts.newChat)) {
			event.preventDefault();
			void createSession();
			return;
		}
		if (event.key === 'Escape' && miniShellStore.sidebarOpen) {
			event.preventDefault();
			miniShellStore.closeSidebar();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="mini-shell">
	{@render children()}
	{#if miniShellStore.sidebarOpen}
		<button class="mini-sidebar-backdrop" type="button" aria-label="Close chats" onclick={() => miniShellStore.closeSidebar()} transition:fade={{ duration: 180, easing: cubicOut }}></button>
		<div class="mini-sidebar-drawer" transition:fly={{ x: -260, opacity: 0, duration: 220, easing: cubicOut }}>
			<MiniSessionSidebar bind:this={sidebarRef} onClose={() => miniShellStore.closeSidebar()} onSelectSession={selectSession} onNewChat={() => void createSession()} />
		</div>
	{/if}
</div>

<style>
	.mini-shell {
		position: relative;
		width: 100vw;
		height: 100vh;
		overflow: hidden;
	}

	.mini-sidebar-backdrop {
		position: absolute;
		inset: 0;
		z-index: 70;
		border: 0;
		background: rgba(0, 0, 0, 0.18);
		cursor: default;
	}

	.mini-sidebar-drawer {
		position: absolute;
		inset: 0 auto 0 0;
		z-index: 80;
		width: min(88vw, 330px);
	}
</style>
