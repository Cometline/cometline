<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import AppShell from '$lib/components/AppShell.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { settingsStore, readHasDismissedSetupWizardSync } from '$lib/stores/settings.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { heroComposerCssVars } from '$lib/hero-composer-appearance';
	import { ensureWorkspace, listAllSessions } from '$lib/client/cometmind';
	import { startJobNotificationPoller } from '$lib/jobs/job-notifications';
	import { startStorageRetentionSync } from '$lib/retention/storage-retention-sync';

	let { children } = $props();

	let settingsLoaded = $state(false);
	let isMiniRoute = $derived(
		page.url.pathname === '/mini' || page.url.pathname.startsWith('/mini/')
	);
	// Prevents the setup wizard from re-opening after the user skips it
	// within the same session (in-memory guard). The durable guard is
	// hasDismissedSetupWizard persisted in settings.
	let setupAutoTriggered = false;
	// Fast synchronous read so the very first effect tick already knows
	// whether the user previously dismissed the wizard.
	let dismissedSetupSync = readHasDismissedSetupWizardSync();

	onMount(() => {
		connectionState.startPolling();
		let stopStorageRetentionSync: (() => void) | null = null;
		const stopJobNotifications = startJobNotificationPoller({
			getSettings: () => settingsStore.settings.cometmind.jobs.notifications,
			onNotify: (title, body) => {
				window.electronAPI?.notifyJob?.({ title, body });
			}
		});
		void settingsStore.load().then(() => {
			settingsLoaded = true;
			stopStorageRetentionSync = startStorageRetentionSync(
				() => settingsStore.settings.cometmind.storage
			);
			// The sync localStorage read in shell.svelte.ts already sets introOpen
			// correctly for the first frame. This IPC result is the authoritative
			// source and handles edge cases:
			// - localStorage cleared but JSON file still has hasSeenIntro=true
			//   → close any intro that the sync read left open.
			// - Fresh install with no localStorage → hasSeenIntro=false
			//   → intro already open; openIntro() is a no-op.
			if (settingsStore.settings.app.hasSeenIntro) {
				shellStore.closeIntro();
			} else {
				shellStore.openIntro();
			}
		});
		void initializeWorkspace();
		return () => {
			connectionState.stopPolling();
			stopJobNotifications();
			stopStorageRetentionSync?.();
		};
	});

	// Auto-open the setup wizard once when the intro has finished and the user
	// hasn't completed setup. Skipping the wizard sets hasDismissedSetupWizard,
	// which is read synchronously on startup and authoritative once settings load.
	$effect(() => {
		if (!settingsLoaded || shellStore.introOpen || shellStore.setupOpen) return;
		if (setupAutoTriggered) return;
		// Honour the persisted dismissal (from settings) or the fast sync read.
		if (
			settingsStore.settings.app.hasDismissedSetupWizard ||
			settingsStore.settings.app.hasCompletedSetup ||
			dismissedSetupSync
		)
			return;
		setupAutoTriggered = true;
		shellStore.openSetup();
	});

	$effect(() => {
		const vars = heroComposerCssVars(settingsStore.settings.appearance.heroComposer);
		const root = document.documentElement;
		for (const [key, value] of Object.entries(vars)) {
			root.style.setProperty(key, value);
		}
	});

	// Apply the persisted web/file panel width. 0 means "use the CSS default".
	$effect(() => {
		const width = settingsStore.settings.app.webPanelWidth;
		const root = document.documentElement;
		if (width > 0) {
			const max = Math.round(window.innerWidth * 0.75);
			const clamped = Math.min(Math.max(width, 320), Math.max(320, max));
			root.style.setProperty('--web-panel-width', `${clamped}px`);
		} else {
			root.style.removeProperty('--web-panel-width');
		}
	});

	$effect(() => {
		if (connectionState.status !== 'ready') return;
		if (shellStore.bootMessage) {
			shellStore.setBootMessage('');
		}
		if (!sessionsLoaded) {
			sessionsLoaded = true;
			void loadSessions();
		}
	});

	let sessionsLoaded = false;
	let lastEnsuredWorkspace = '';

	$effect(() => {
		const workspacePath = shellStore.workspacePath;
		if (!workspacePath || workspacePath === '/') return;
		if (connectionState.status !== 'ready') return;
		// Register the workspace in the background (needed for forks / new
		// chats) without blocking the session list or transcript loads. Only
		// re-register when the path actually changes.
		if (workspacePath !== lastEnsuredWorkspace) {
			lastEnsuredWorkspace = workspacePath;
			void ensureWorkspace(workspacePath).catch(() => {});
		}
	});

	async function initializeWorkspace() {
		try {
			const workspacePath = (await window.electronAPI?.getWorkspacePath?.()) ?? '/';
			shellStore.initializeDefaultWorkspace(workspacePath);
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
			if (connectionState.status === 'connecting') {
				// Let the runtime overlay own startup copy while the sidecar is still
				// warming up; we'll retry automatically once it reports healthy.
				sessionsLoaded = false;
				return;
			}
			// Allow a later workspace/effect tick to retry (e.g. backend not
			// healthy yet at startup).
			sessionsLoaded = false;
			shellStore.setBootMessage(
				err instanceof Error ? err.message : 'Failed to load sessions'
			);
		}
	}
</script>

{#if isMiniRoute}
	{@render children()}
{:else}
	<AppShell>
		{@render children()}
	</AppShell>
{/if}
