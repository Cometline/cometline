import { settingsStore } from '$lib/stores/settings.svelte';
import type { ShortcutAction, ShortcutBinding } from '$lib/types';
import type { SettingsPanelControllerDeps } from './settings-panel-types';

export function createSettingsPanelPreferences(
	deps: Pick<SettingsPanelControllerDeps, 'getDraft' | 'setDraft' | 'settingsController'>
) {
	let screenCaptureStatus = $state('unknown');

	function loadElectronPreferences(api: NonNullable<typeof window.electronAPI>) {
		void api.getOpenAtLogin?.().then((current) => {
			if (current) {
				deps.setDraft({
					...deps.getDraft(),
					app: { ...deps.getDraft().app, openAtLogin: current.openAtLogin }
				});
			}
		});
		void api.getScreenCaptureAccess?.().then((current) => {
			if (!current) return;
			screenCaptureStatus = current.status ?? 'unknown';
			deps.setDraft({
				...deps.getDraft(),
				app: {
					...deps.getDraft().app,
					screenCapturePreferred: Boolean(current.preferred)
				}
			});
		});
	}

	function updateShortcut(action: ShortcutAction, binding: ShortcutBinding) {
		const draft = deps.getDraft();
		const shortcuts = {
			...draft.shortcuts,
			[action]: binding
		};
		deps.setDraft({
			...draft,
			shortcuts
		});
		void settingsStore.saveShortcuts(shortcuts).then(() => {
			deps.settingsController.status = 'Shortcut updated and saved.';
		});
	}

	async function setOpenAtLogin(enabled: boolean) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, openAtLogin: enabled } });
		const result = await window.electronAPI?.setOpenAtLogin?.(enabled);
		if (!result) return;

		deps.setDraft({
			...deps.getDraft(),
			app: { ...deps.getDraft().app, openAtLogin: result.openAtLogin }
		});

		if (result.openedSettings) {
			const devNote = result.isDev ? ' In dev mode it may appear as Electron.' : '';
			deps.settingsController.status = result.needsApproval
				? `macOS needs your approval in System Settings → Login Items. Enable Cometline there.${devNote}`
				: `Opened System Settings → Login Items. Confirm Cometline is allowed to open at login.${devNote}`;
		} else if (!enabled) {
			deps.settingsController.status = 'Cometline will no longer open at login.';
		} else if (result.openAtLogin) {
			deps.settingsController.status = 'Cometline will open at login.';
		}
	}

	async function setScreenCapturePreferred(enabled: boolean) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, screenCapturePreferred: enabled } });
		const result = await window.electronAPI?.setScreenCapturePreferred?.(enabled);
		if (!result) return;

		screenCaptureStatus = result.status ?? 'unknown';
		deps.setDraft({
			...deps.getDraft(),
			app: {
				...deps.getDraft().app,
				screenCapturePreferred: Boolean(result.preferred)
			}
		});

		if (result.openedSettings) {
			deps.settingsController.status =
				result.message ??
				'Opened System Settings → Screen & System Audio Recording. Enable Cometline there.';
		} else if (!enabled) {
			deps.settingsController.status = 'Screen capture preference turned off.';
		} else if (result.status === 'granted') {
			deps.settingsController.status = 'Screen capture is allowed.';
		} else if (result.message) {
			deps.settingsController.status = result.message;
		}
	}

	async function openScreenCaptureSettings() {
		const opened = await window.electronAPI?.openScreenCaptureSettings?.();
		if (opened) {
			deps.settingsController.status =
				'Opened System Settings → Screen & System Audio Recording.';
		}
		const current = await window.electronAPI?.getScreenCaptureAccess?.();
		if (current) screenCaptureStatus = current.status ?? screenCaptureStatus;
	}

	async function setConfirmCloseOnCmdW(enabled: boolean) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, confirmCloseOnCmdW: enabled } });
		try {
			await settingsStore.saveConfirmCloseOnCmdW(enabled);
			deps.settingsController.status = enabled
				? 'Will ask for confirmation before closing with ⌘W.'
				: 'Closing with ⌘W will hide without confirmation.';
		} catch (err) {
			deps.setDraft({
				...deps.getDraft(),
				app: {
					...deps.getDraft().app,
					confirmCloseOnCmdW: settingsStore.settings.app.confirmCloseOnCmdW
				}
			});
			deps.settingsController.status =
				err instanceof Error ? err.message : 'Failed to save close preference';
		}
	}

	async function setConfirmBeforeDeletingChats(enabled: boolean) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, confirmBeforeDeletingChats: enabled } });
		try {
			await settingsStore.saveConfirmBeforeDeletingChats(enabled);
			deps.settingsController.status = enabled
				? 'Will ask for confirmation before deleting chats.'
				: 'Chats will delete without confirmation.';
		} catch (err) {
			deps.setDraft({
				...deps.getDraft(),
				app: {
					...deps.getDraft().app,
					confirmBeforeDeletingChats:
						settingsStore.settings.app.confirmBeforeDeletingChats
				}
			});
			deps.settingsController.status =
				err instanceof Error
					? err.message
					: 'Failed to save delete confirmation preference';
		}
	}

	async function setConfirmBeforeDeletingMedia(enabled: boolean) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, confirmBeforeDeletingMedia: enabled } });
		try {
			await settingsStore.saveConfirmBeforeDeletingMedia(enabled);
			deps.settingsController.status = enabled
				? 'Will ask for confirmation before deleting Gallery media.'
				: 'Gallery media will delete without confirmation.';
		} catch (err) {
			deps.setDraft({
				...deps.getDraft(),
				app: {
					...deps.getDraft().app,
					confirmBeforeDeletingMedia:
						settingsStore.settings.app.confirmBeforeDeletingMedia
				}
			});
			deps.settingsController.status =
				err instanceof Error ? err.message : 'Failed to save media delete preference';
		}
	}

	async function setFileSearchSource(source: 'wiki' | 'workspace') {
		const next = source === 'workspace' ? 'workspace' : 'wiki';
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, fileSearchSource: next } });
		try {
			await settingsStore.saveFileSearchSource(next);
			deps.settingsController.status =
				next === 'wiki'
					? 'File search defaults to wiki.'
					: 'File search defaults to workspace.';
		} catch (err) {
			deps.setDraft({
				...deps.getDraft(),
				app: {
					...deps.getDraft().app,
					fileSearchSource: settingsStore.settings.app.fileSearchSource
				}
			});
			deps.settingsController.status =
				err instanceof Error ? err.message : 'Failed to save file search preference';
		}
	}

	return {
		get screenCaptureStatus() {
			return screenCaptureStatus;
		},
		loadElectronPreferences,
		updateShortcut,
		setOpenAtLogin,
		setScreenCapturePreferred,
		openScreenCaptureSettings,
		setConfirmCloseOnCmdW,
		setConfirmBeforeDeletingChats,
		setConfirmBeforeDeletingMedia,
		setFileSearchSource
	};
}
