<script lang="ts">
	import { Globe, Play, Power, SquareTerminal, X } from '@lucide/svelte';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import TerminalInstance from '$lib/components/TerminalInstance.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';
	import { TERMINAL_THEME_PRESETS } from '$lib/terminal-appearance';

	let terminateConfirmOpen = $state(false);
	let starting = $state(false);
	let lastStartRequest = 0;
	const panelOpen = $derived(shellStore.terminalPanelOpen);
	const activeSession = $derived(sessionStore.current);
	const activeTerminal = $derived(
		activeSession ? terminalStore.getSnapshot(activeSession.id) : null
	);
	const terminalIds = $derived(terminalStore.sessionIds);
	const terminalTheme = $derived(
		TERMINAL_THEME_PRESETS[settingsStore.settings.appearance.terminal.theme].colors
	);

	async function startTerminal() {
		if (!activeSession || starting) return;
		starting = true;
		try {
			await terminalStore.start(activeSession.id, activeSession.workspace_path);
		} finally {
			starting = false;
		}
	}

	async function confirmTerminate() {
		if (!activeSession) return;
		terminateConfirmOpen = false;
		await terminalStore.terminate(activeSession.id);
	}

	$effect(() => {
		const request = shellStore.terminalFocusRequestId;
		if (!request || request === lastStartRequest || !panelOpen || !activeSession || starting)
			return;
		if (activeTerminal?.status === 'running') return;
		lastStartRequest = request;
		void startTerminal();
	});
</script>

<div class="terminal-panel" class:open={panelOpen} aria-hidden={!panelOpen}>
	<div
		class="terminal-panel-inner content-panel-surface"
		class:pane-focus-active={shellStore.focusedPane === 'terminal' && panelOpen}
	>
		<header class="terminal-panel-toolbar">
			<div class="surface-switcher" role="group" aria-label="Workspace panel surface">
				<Tooltip label="Web" action="openWebPanel">
					<button
						type="button"
						class="icon-button"
						onclick={() => shellStore.openWebPanelFromShortcut()}
						aria-label="Open web panel"
					>
						<Globe size={16} />
					</button>
				</Tooltip>
				<Tooltip label="Terminal" action="openTerminal">
					<button
						type="button"
						class="icon-button active"
						onclick={() => shellStore.requestTerminalFocus()}
						aria-label="Focus terminal"
					>
						<SquareTerminal size={16} />
					</button>
				</Tooltip>
			</div>
			<div class="terminal-title">
				{#if activeTerminal?.status === 'running'}
					<span>Terminal</span>
				{:else if activeTerminal?.status === 'exited'}
					<span>Terminal exited</span>
				{:else}
					<span>Terminal</span>
				{/if}
			</div>
			<div class="terminal-actions">
				{#if activeTerminal?.status === 'running'}
					<button
						type="button"
						class="icon-button"
						onclick={() => (terminateConfirmOpen = true)}
						aria-label="Terminate terminal"
						title="Terminate terminal"
					>
						<Power size={16} />
					</button>
				{:else}
					<button
						type="button"
						class="icon-button"
						onclick={() => void startTerminal()}
						disabled={!activeSession || starting}
						aria-label="Start terminal"
						title="Start terminal"
					>
						<Play size={16} />
					</button>
				{/if}
				<button
					type="button"
					class="icon-button close-button"
					onclick={() => shellStore.closeWorkspacePanel()}
					aria-label="Close panel"
					title="Close panel"
				>
					<X size={16} />
				</button>
			</div>
		</header>
		<div class="terminal-panel-content" style:background={terminalTheme.background}>
			{#if !activeSession}
				<div class="terminal-empty">Open a chat to start a terminal.</div>
			{:else if !activeTerminal}
				<div class="terminal-empty">
					<p>Start a terminal in this chat's workspace.</p>
					<button type="button" onclick={() => void startTerminal()} disabled={starting}
						>{starting ? 'Starting…' : 'Start terminal'}</button
					>
				</div>
			{/if}
			{#each terminalIds as sessionId (sessionId)}
				{@const terminal = terminalStore.getSnapshot(sessionId)}
				{#key terminal?.generation ?? 0}
					<TerminalInstance
						{sessionId}
						active={panelOpen && activeSession?.id === sessionId}
						focusRequestId={shellStore.terminalFocusRequestId}
					/>
				{/key}
			{/each}
		</div>
	</div>
</div>

<ConfirmActionModal
	open={terminateConfirmOpen}
	title="Terminate terminal?"
	description="This will stop the shell and every program started from it, including tmux, development servers, and SSH connections. This cannot be undone."
	confirmLabel="Terminate terminal"
	onCancel={() => (terminateConfirmOpen = false)}
	onConfirm={() => void confirmTerminate()}
/>

<style>
	.terminal-panel {
		flex: 0 0 auto;
		width: 0;
		min-width: 0;
		height: 100%;
		overflow: hidden;
		pointer-events: none;
		box-sizing: border-box;
		transition: width var(--duration-fast) var(--ease-smooth);
	}
	.terminal-panel.open {
		width: var(--web-panel-slot-width);
		max-width: 100%;
		min-width: 0;
		flex-shrink: 1;
		pointer-events: auto;
	}
	.terminal-panel-inner {
		width: var(--web-panel-width);
		height: calc(100% - (2 * var(--content-panel-inset)));
		display: flex;
		flex-direction: column;
		margin: var(--content-panel-inset);
		margin-left: 0;
		overflow: hidden;
		box-sizing: border-box;
		transition: width var(--duration-fast) var(--ease-smooth);
	}
	.terminal-panel-toolbar {
		display: flex;
		align-items: center;
		min-height: var(--panel-header-height);
		padding: 0 10px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(250, 250, 249, 0.95);
	}
	.surface-switcher,
	.terminal-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-shrink: 0;
	}
	.terminal-title {
		flex: 1;
		min-width: 0;
		padding: 0 8px;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 600;
	}
	.icon-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-main);
		cursor: pointer;
	}
	.icon-button:hover:not(:disabled),
	.icon-button.active {
		background: rgba(15, 23, 42, 0.06);
	}
	.icon-button:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}
	.terminal-panel-content {
		position: relative;
		display: flex;
		flex: 1;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
	}
	.terminal-empty {
		display: grid;
		place-content: center;
		gap: 12px;
		width: 100%;
		color: #d4d4d8;
		font-size: 13px;
		text-align: center;
	}
	.terminal-empty p {
		margin: 0;
	}
	.terminal-empty button {
		border: none;
		border-radius: 8px;
		padding: 8px 12px;
		background: #f4f4f5;
		color: #18181b;
		font: inherit;
		font-weight: 650;
		cursor: pointer;
	}
</style>
