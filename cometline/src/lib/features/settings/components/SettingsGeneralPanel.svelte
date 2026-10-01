<script lang="ts">
	import SettingsToggle from './SettingsToggle.svelte';
	import StorageRetentionSection from './general/StorageRetentionSection.svelte';
	import DataBackupSection from './general/DataBackupSection.svelte';
	import type { CometMindStorageSettings, FileSearchSource } from '$lib/features/settings/schema';
	import {
		miniWindowTimeoutFromInput,
		screenCaptureStatusLabel
	} from '$lib/features/settings/general-panel-inputs';

	let {
		openAtLogin = $bindable(false),
		screenCapturePreferred = $bindable(false),
		screenCaptureStatus = $bindable('unknown'),
		confirmCloseOnCmdW = $bindable(true),
		confirmBeforeDeletingChats = $bindable(true),
		confirmBeforeDeletingMedia = $bindable(true),
		fileSearchSource = $bindable<FileSearchSource>('wiki'),
		miniWindowInactivityTimeoutMinutes = $bindable(30),
		storage = $bindable<CometMindStorageSettings>(),
		onOpenAtLoginChange,
		onScreenCapturePreferredChange,
		onOpenScreenCaptureSettings,
		onConfirmCloseOnCmdWChange,
		onConfirmBeforeDeletingChatsChange,
		onConfirmBeforeDeletingMediaChange,
		onFileSearchSourceChange
	}: {
		openAtLogin: boolean;
		screenCapturePreferred: boolean;
		screenCaptureStatus: string;
		confirmCloseOnCmdW: boolean;
		confirmBeforeDeletingChats: boolean;
		confirmBeforeDeletingMedia: boolean;
		fileSearchSource: FileSearchSource;
		miniWindowInactivityTimeoutMinutes: number;
		storage: CometMindStorageSettings;
		onOpenAtLoginChange?: (enabled: boolean) => void | Promise<void>;
		onScreenCapturePreferredChange?: (enabled: boolean) => void | Promise<void>;
		onOpenScreenCaptureSettings?: () => void | Promise<void>;
		onConfirmCloseOnCmdWChange?: (enabled: boolean) => void | Promise<void>;
		onConfirmBeforeDeletingChatsChange?: (enabled: boolean) => void | Promise<void>;
		onConfirmBeforeDeletingMediaChange?: (enabled: boolean) => void | Promise<void>;
		onFileSearchSourceChange?: (source: FileSearchSource) => void | Promise<void>;
	} = $props();

	const screenStatusLabel = $derived(screenCaptureStatusLabel(screenCaptureStatus));

	function onMiniWindowTimeoutInput(event: Event) {
		miniWindowInactivityTimeoutMinutes = miniWindowTimeoutFromInput(event);
	}
</script>

<section class="general-panel settings-panel-frame">
	<div class="settings-panel-body">
		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Startup</h3>
				<p>Control how Cometline launches on your Mac.</p>
			</div>
			<SettingsToggle
				label="Open at login"
				description="Launch Cometline when you sign in. On macOS 13+, you may need to approve it in System Settings → Login Items."
				bind:checked={openAtLogin}
				disabled={!window.electronAPI?.setOpenAtLogin}
				onchange={onOpenAtLoginChange}
			/>
			<SettingsToggle
				label="Confirm before deleting chats"
				description="Ask for confirmation before permanently deleting a chat."
				bind:checked={confirmBeforeDeletingChats}
				onchange={onConfirmBeforeDeletingChatsChange}
			/>
			<SettingsToggle
				label="Confirm before deleting Gallery media"
				description="Ask for confirmation before permanently deleting an image or video from Gallery."
				bind:checked={confirmBeforeDeletingMedia}
				onchange={onConfirmBeforeDeletingMediaChange}
			/>
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Screen & system audio</h3>
				<p>
					Allow Cometline to capture the screen so the agent can take screenshots and show
					them inline in chat. You can change this later; macOS may still ask for approval
					in System Settings.
				</p>
			</div>
			<SettingsToggle
				label="Enable screen capture"
				description="Prefers Screen & System Audio Recording so screenshots can appear inline in chat."
				bind:checked={screenCapturePreferred}
				disabled={!window.electronAPI?.setScreenCapturePreferred}
				onchange={onScreenCapturePreferredChange}
			/>
			<p class="permission-status">{screenStatusLabel}</p>
			{#if window.electronAPI?.openScreenCaptureSettings}
				<button
					type="button"
					class="settings-secondary-btn"
					onclick={() => void onOpenScreenCaptureSettings?.()}
				>
					Open System Settings…
				</button>
			{/if}
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Window</h3>
				<p>Control what happens when you close the main window with ⌘W.</p>
			</div>
			<SettingsToggle
				label="Confirm before closing"
				description="Ask for confirmation when closing the main window with ⌘W. The window hides to the menu bar instead of quitting."
				bind:checked={confirmCloseOnCmdW}
				onchange={onConfirmCloseOnCmdWChange}
			/>
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>File search</h3>
				<p>Default source for the ⌘P quick-open file search modal.</p>
			</div>
			<div class="source-toggle" role="group" aria-label="File search source">
				<button
					type="button"
					class="source-toggle-btn"
					class:active={fileSearchSource === 'wiki'}
					onclick={() => {
						fileSearchSource = 'wiki';
						void onFileSearchSourceChange?.('wiki');
					}}
				>
					Wiki
				</button>
				<button
					type="button"
					class="source-toggle-btn"
					class:active={fileSearchSource === 'workspace'}
					onclick={() => {
						fileSearchSource = 'workspace';
						void onFileSearchSourceChange?.('workspace');
					}}
				>
					Workspace
				</button>
			</div>
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Mini window</h3>
				<p>Control when the compact window starts a fresh rolling session.</p>
			</div>
			<label class="field">
				<span>Mini window reset timeout (minutes)</span>
				<input
					type="number"
					min="1"
					max="1440"
					step="1"
					value={miniWindowInactivityTimeoutMinutes}
					oninput={onMiniWindowTimeoutInput}
				/>
				<small>
					After the mini window stays hidden long enough, the next hotkey open starts a
					new rolling session. Current setting: {miniWindowInactivityTimeoutMinutes} minute{miniWindowInactivityTimeoutMinutes ===
					1
						? ''
						: 's'}.
				</small>
			</label>
		</div>

		<StorageRetentionSection bind:storage />

		<DataBackupSection bind:storage />
	</div>
</section>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 13px;
		color: var(--text-main);
	}

	.field input {
		max-width: 160px;
	}

	.field small {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.permission-status {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.settings-secondary-btn {
		align-self: flex-start;
		margin-top: 4px;
		padding: 6px 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--text-muted) 28%, transparent);
		background: transparent;
		color: var(--text-main);
		font-size: 12px;
		cursor: pointer;
	}

	.settings-secondary-btn:hover {
		background: color-mix(in srgb, var(--text-muted) 10%, transparent);
	}

	.source-toggle {
		display: flex;
		gap: 2px;
		width: fit-content;
		padding: 2px;
		border-radius: 8px;
		background: var(--surface-muted, rgba(0, 0, 0, 0.04));
	}

	.source-toggle-btn {
		border: none;
		border-radius: 6px;
		padding: 6px 12px;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 550;
		cursor: pointer;
	}

	.source-toggle-btn.active {
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
		box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.06);
	}
</style>
