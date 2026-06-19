<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import AppShell from '$lib/components/AppShell.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { heroComposerCssVars } from '$lib/hero-composer-appearance';
	import { ensureWorkspace, listAllSessions } from '$lib/client/cometmind';

	let { children } = $props();

	onMount(() => {
		connectionState.startPolling();
		void settingsStore.load().then(() => {
			// First launch: play the cinematic intro once.
			if (!settingsStore.settings.app.hasSeenIntro) {
				shellStore.openIntro();
			}
		});
		void initializeWorkspace();
		return () => connectionState.stopPolling();
	});

	$effect(() => {
		const vars = heroComposerCssVars(settingsStore.settings.appearance.heroComposer);
		const root = document.documentElement;
		for (const [key, value] of Object.entries(vars)) {
			root.style.setProperty(key, value);
		}
	});

	let sessionsLoaded = false;
	let lastEnsuredWorkspace = '';

	$effect(() => {
		const workspacePath = shellStore.workspacePath;
		if (!workspacePath || workspacePath === '/') return;
		// Register the workspace in the background (needed for forks / new
		// chats) without blocking the session list or transcript loads. Only
		// re-register when the path actually changes.
		if (workspacePath !== lastEnsuredWorkspace) {
			lastEnsuredWorkspace = workspacePath;
			void ensureWorkspace(workspacePath).catch(() => {});
		}
		// The session list spans every workspace, so it does not depend on the
		// current workspace path — load it once. Subsequent new/fork/delete
		// mutate sessionStore locally and stay in sync without a refetch.
		if (!sessionsLoaded) {
			sessionsLoaded = true;
			void loadSessions();
		}
	});

	async function initializeWorkspace() {
		try {
			const workspacePath = (await window.electronAPI?.getWorkspacePath?.()) ?? '/';
			shellStore.commitWorkspace(workspacePath);
		} catch (err) {
			shellStore.setBootMessage(
				err instanceof Error ? err.message : 'Failed to initialize workspace'
			);
		}
	}

	async function loadSessions() {
		try {
			const result = await listAllSessions();
			sessionStore.setSessions(result.sessions);
			shellStore.setBootMessage('');
		} catch (err) {
			// Allow a later workspace/effect tick to retry (e.g. backend not
			// healthy yet at startup).
			sessionsLoaded = false;
			shellStore.setBootMessage(
				err instanceof Error ? err.message : 'Failed to load sessions'
			);
		}
	}
</script>

<AppShell>
	{@render children()}
</AppShell>
