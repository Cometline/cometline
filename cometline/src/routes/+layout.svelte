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
	import {
		notifyBackgroundRunFinished,
		notifyConnectionChange,
		notifyJobActivity,
		notifyNewInboxMessage,
		startSkillDraftToastWatch
	} from '$lib/notifications/activity-toasts';
	import { startStorageRetentionSync } from '$lib/retention/storage-retention-sync';
	import { createBootController } from '$lib/boot/boot-controller';
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
	let previousConnectionStatus = $state(connectionState.status);

	$effect(() => {
		const next = connectionState.status;
		if (!isMiniRoute && !isSettingsRoute) notifyConnectionChange(previousConnectionStatus, next);
		previousConnectionStatus = next;
	});
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
			updateSession: sessionStore.updateSession,
			isStreamingFor: chatStore.isStreamingFor,
			hasLocalStream: chatStore.hasLocalStream
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
				if (!isMiniRoute && !isSettingsRoute) void notifyNewInboxMessage(event.id);
			}
			if (event.type === 'run_finished' && !isMiniRoute && !isSettingsRoute) {
				void notifyBackgroundRunFinished(event.session_id, chatStore.sessionID);
			}
			if (event.type === 'inbox_message_archived') {
				inboxStore.applyArchived(event.id, event.open_count);
			}
		}, () => reconcileActiveSession(runtimeEventDeps));
		void inboxStore.refreshSummary();
		let skillDraftsTimer: ReturnType<typeof setInterval> | null = null;
		let stopSkillDraftToasts = () => {};
		if (!isMiniRoute && !isSettingsRoute) {
			void skillDraftsStore.refresh();
			skillDraftsTimer = setInterval(() => {
				void skillDraftsStore.refresh();
			}, 30_000);
			stopSkillDraftToasts = startSkillDraftToastWatch({
				isReviewOpen: () =>
					page.url.pathname === '/skills' || page.url.pathname.startsWith('/skills/')
			});
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
						onNotify: (title, body, job) => {
							window.electronAPI?.notifyJob?.({ title, body });
							if (title === 'Job completed' && job) {
								notifyJobActivity(
									{ kind: 'completed', id: job.id, description: job.description },
									settingsStore.settings.cometmind.jobs.notifications
								);
							}
							if (title === 'Job blocked' && job) {
								notifyJobActivity(
									{ kind: 'blocked', id: job.id, description: job.description },
									settingsStore.settings.cometmind.jobs.notifications
								);
							}
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
			stopSkillDraftToasts();
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
	// DOM adapter. Not boot state.
	$effect(() => {
		const vars = heroComposerCssVars(settingsStore.settings.appearance.heroComposer);
		const root = document.documentElement;
		for (const [key, value] of Object.entries(vars)) {
			root.style.setProperty(key, value);
		}
	});

	// AppShell is the only writer of --workspace-panel-width. A second seed
	// here, keyed on window.innerWidth, shrink-then-grew the panel when the
	// sidebar opened.

	const boot = createBootController({
		loadSessions: () => {
			void loadSessions();
		},
		refreshModelLimits: () => {
			void settingsStore.refreshModelLimits();
		},
		ensureWorkspace: (path) => {
			void ensureWorkspace(path).catch(() => {});
		},
		watchWorkspace: (path) => {
			void window.electronAPI?.watchWorkspace?.(path);
		},
		openSetup: () => shellStore.openSetup(),
		clearBootMessage: () => {
			if (shellStore.bootMessage) shellStore.setBootMessage('');
		}
	});

	// One adapter. Decisions live in the boot module.
	$effect(() => {
		boot.sync({
			connectionReady: connectionState.status === 'ready',
			settingsLoaded,
			introOpen: shellStore.introOpen,
			setupOpen: shellStore.setupOpen,
			setupDismissed:
				settingsStore.settings.app.hasDismissedSetupWizard || dismissedSetupSync,
			setupCompleted: settingsStore.settings.app.hasCompletedSetup,
			workspacePath: shellStore.workspacePath,
			skipWatch: isMiniRoute || isSettingsRoute
		});
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
				boot.sessionsFailed();
				return;
			}
			// Allow a later ready tick to retry (e.g. backend not healthy yet).
			boot.sessionsFailed();
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
