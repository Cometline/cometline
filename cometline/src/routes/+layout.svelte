<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import AppShell from '$lib/components/AppShell.svelte';
	import MiniShell from '$lib/components/MiniShell.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { settingsStore, readHasDismissedSetupWizardSync } from '$lib/stores/settings.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { personaAvatarCache } from '$lib/personas/avatar-cache.svelte';
	import { heroComposerCssVars } from '$lib/hero-composer-appearance';
	import {
		ensureWorkspace,
		getSession,
		listAllSessions,
		startRuntimeEventStream
	} from '$lib/client/cometmind';
	import { memoryToastStore } from '$lib/stores/memory-toasts.svelte';
	import { inboxStore } from '$lib/stores/inbox.svelte';
	import { skillDraftsStore } from '$lib/stores/skill-drafts.svelte';
	import { startJobNotificationPoller } from '$lib/jobs/job-notifications';
	import { startStorageRetentionSync } from '$lib/retention/storage-retention-sync';
	import { resolveWorkspacePanelRatio, widthFromRatio } from '$lib/layout/workspace-panel-width';
	import { applyWorkspaceChange, refreshWorkspace } from '$lib/workspace/workspace-change.svelte';
	import { chatStore } from '$lib/stores/chat.svelte';
	import {
		applySessionRuntimeEvent,
		reconcileActiveSession
	} from '$lib/sessions/session-runtime-events';

	let { children } = $props();

	let settingsLoaded = $state(false);
	let isMiniRoute = $derived(
		page.url.pathname === '/mini' || page.url.pathname.startsWith('/mini/')
	);
	let isSettingsRoute = $derived(
		page.url.pathname === '/settings' || page.url.pathname.startsWith('/settings/')
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
		const runtimeEventDeps = {
			getActiveSessionId: () => chatStore.sessionID,
			setRunning: sessionStore.setRunning,
			refreshTranscript: chatStore.refreshTranscript,
			resumeRun: chatStore.resumeRun,
			refreshSession: getSession,
			updateSession: sessionStore.updateSession
		};
		const stopRuntimeEvents = startRuntimeEventStream((event) => {
			void applySessionRuntimeEvent(event, runtimeEventDeps);
			if (event.type === 'memory_updated') {
				memoryToastStore.add(event.changes);
			}
			if (event.type === 'memory_compaction_completed') {
				memoryToastStore.addCompaction(event);
			}
			if (event.type === 'inbox_message_created') {
				inboxStore.applyCreated(event.id, event.open_count);
			}
			if (event.type === 'inbox_message_archived') {
				inboxStore.applyArchived(event.id, event.open_count);
			}
		}, () => reconcileActiveSession(runtimeEventDeps));
		void inboxStore.refreshSummary();
		let skillDraftsTimer: ReturnType<typeof setInterval> | null = null;
		if (!isMiniRoute && !isSettingsRoute) {
			void skillDraftsStore.refresh();
			skillDraftsTimer = setInterval(() => {
				void skillDraftsStore.refresh();
			}, 30_000);
		}
		let stopStorageRetentionSync: (() => void) | null = null;
		// Mini/settings are separate BrowserWindows that share this layout. Only the
		// main window should poll — otherwise each alive window fires the same
		// desktop notification when a job transitions.
		const stopJobNotifications =
			isMiniRoute || isSettingsRoute
				? () => {}
				: startJobNotificationPoller({
						getSettings: () => settingsStore.settings.cometmind.jobs.notifications,
						onNotify: (title, body) => {
							window.electronAPI?.notifyJob?.({ title, body });
						}
					});
		const unsubscribeSettingsChanged = window.electronAPI?.onProviderSettingsChanged?.(
			(settings) => {
				settingsStore.apply(settings);
			}
		);
		// A custom persona's avatar image was replaced (same id) — drop the stale
		// cached data URL so intro/avatars re-fetch the new image.
		const unsubscribePersonaAvatar = window.electronAPI?.onPersonaAvatarChanged?.(
			(personaId) => {
				personaAvatarCache.invalidate(personaId);
			}
		);
		const unsubscribeWorkspaceChanged =
			window.electronAPI?.onWorkspaceChanged?.(applyWorkspaceChange);
		const refreshOnFocus = () => {
			if (isMiniRoute || isSettingsRoute) return;
			refreshWorkspace(shellStore.workspacePath);
		};
		window.addEventListener('focus', refreshOnFocus);
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
			stopRuntimeEvents();
			stopJobNotifications();
			if (skillDraftsTimer) clearInterval(skillDraftsTimer);
			stopStorageRetentionSync?.();
			unsubscribeSettingsChanged?.();
			unsubscribePersonaAvatar?.();
			unsubscribeWorkspaceChanged?.();
			window.removeEventListener('focus', refreshOnFocus);
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

	// Seed --workspace-panel-width once when persisted prefs first become available.
	// After that, AppShell exclusively owns live updates against the content-row
	// — a second writer keyed on window.innerWidth caused shrink-then-grow when
	// opening the sidebar.
	let didSeedWorkspacePanelWidth = false;
	$effect(() => {
		const prefs = {
			workspacePanelRatio: settingsStore.settings.app.workspacePanelRatio,
			workspacePanelWidth: settingsStore.settings.app.workspacePanelWidth
		};
		if (didSeedWorkspacePanelWidth) return;
		if (prefs.workspacePanelRatio <= 0 && prefs.workspacePanelWidth <= 0) return;
		didSeedWorkspacePanelWidth = true;
		const ratio = resolveWorkspacePanelRatio(prefs, window.innerWidth);
		const clamped = widthFromRatio(ratio, window.innerWidth, {
			sidebarOpen: false,
			fullscreen: false
		});
		document.documentElement.style.setProperty('--workspace-panel-width', `${clamped}px`);
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

	$effect(() => {
		if (!settingsLoaded || connectionState.status !== 'ready') return;
		void settingsStore.refreshModelLimits();
	});

	let sessionsLoaded = false;
	let lastEnsuredWorkspace = '';

	$effect(() => {
		const workspacePath = shellStore.workspacePath;
		if (isMiniRoute || isSettingsRoute) return;
		if (!workspacePath || workspacePath === '/') return;
		void window.electronAPI?.watchWorkspace?.(workspacePath);
	});

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
	<MiniShell>
		{@render children()}
	</MiniShell>
{:else if isSettingsRoute}
	{@render children()}
{:else}
	<AppShell>
		{@render children()}
	</AppShell>
{/if}
