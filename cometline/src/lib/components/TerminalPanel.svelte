<script lang="ts">
	import { Play } from '@lucide/svelte';
	import TerminalInstance from '$lib/components/TerminalInstance.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';
	import { TERMINAL_THEME_PRESETS } from '$lib/terminal-appearance';

	let {
		active = false
	}: {
		/** When false the layer is covered; instances stay mounted. */
		active?: boolean;
	} = $props();

	let starting = $state(false);
	let lastStartRequest = 0;
	const activeSession = $derived(sessionStore.current);
	const activeTerminal = $derived(
		activeSession ? terminalStore.getSnapshot(activeSession.id) : null
	);
	const terminalIds = $derived(terminalStore.sessionIds);
	const terminalTheme = $derived(
		TERMINAL_THEME_PRESETS[settingsStore.settings.appearance.terminal.theme].colors
	);

	export async function startTerminal() {
		if (!activeSession || starting) return;
		starting = true;
		try {
			await terminalStore.start(activeSession.id, activeSession.workspace_path);
		} finally {
			starting = false;
		}
	}

	$effect(() => {
		const request = shellStore.terminalFocusRequestId;
		if (!request || request === lastStartRequest || !active || !activeSession || starting)
			return;
		// Consume the focus request even when already running so a later exit
		// cannot see a stale request id and auto-restart the PTY.
		if (activeTerminal?.status === 'running') {
			lastStartRequest = request;
			return;
		}
		lastStartRequest = request;
		void startTerminal();
	});
</script>

<div class="terminal-panel-content" style:background={terminalTheme.background}>
	{#if !activeSession}
		<div class="terminal-empty">Open a chat to start a terminal.</div>
	{:else if !activeTerminal}
		<div class="terminal-empty">
			<p>Start a terminal in this chat's workspace.</p>
			<button type="button" onclick={() => void startTerminal()} disabled={starting}>
				<Play size={14} />
				{starting ? 'Starting…' : 'Start terminal'}
			</button>
		</div>
	{/if}
	{#each terminalIds as sessionId (sessionId)}
		{@const terminal = terminalStore.getSnapshot(sessionId)}
		{#key terminal?.generation ?? 0}
			<TerminalInstance
				{sessionId}
				active={active && activeSession?.id === sessionId}
				focusRequestId={shellStore.terminalFocusRequestId}
			/>
		{/key}
	{/each}
</div>

<style>
	.terminal-panel-content {
		position: relative;
		display: flex;
		flex: 1;
		min-width: 0;
		min-height: 0;
		height: 100%;
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
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		border: none;
		border-radius: 8px;
		padding: 8px 12px;
		background: #f4f4f5;
		color: #18181b;
		font: inherit;
		font-weight: 650;
		cursor: pointer;
	}
	.terminal-empty button:disabled {
		opacity: 0.55;
		cursor: default;
	}
</style>
