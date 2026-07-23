<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import '@xterm/xterm/css/xterm.css';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';
	import { DEFAULT_TERMINAL_FONT_FAMILY, TERMINAL_THEME_PRESETS } from '$lib/terminal-appearance';

	let {
		sessionId,
		active = false,
		focusRequestId = 0
	}: {
		sessionId: string;
		active?: boolean;
		focusRequestId?: number;
	} = $props();

	let host = $state<HTMLDivElement | null>(null);
	let terminal = $state<Terminal | null>(null);
	let fitAddon = $state<FitAddon | null>(null);
	let fitFrame: number | null = null;
	const snapshot = $derived(terminalStore.getSnapshot(sessionId));
	const terminalAppearance = $derived(settingsStore.settings.appearance.terminal);
	const terminalTheme = $derived(TERMINAL_THEME_PRESETS[terminalAppearance.theme].colors);

	function applyAppearance(instance: Terminal) {
		instance.options.fontFamily = DEFAULT_TERMINAL_FONT_FAMILY;
		instance.options.fontSize = terminalAppearance.fontSize;
		instance.options.theme = terminalTheme;
	}

	function fit() {
		if (!active || !fitAddon || !terminal) return;
		try {
			fitAddon.fit();
			void terminalStore.resize(sessionId, terminal.cols, terminal.rows);
		} catch {
			// The panel can be transiently hidden while switching surfaces.
		}
	}

	function scheduleFit() {
		if (fitFrame !== null) cancelAnimationFrame(fitFrame);
		fitFrame = requestAnimationFrame(() => {
			fitFrame = null;
			fit();
		});
	}

	$effect(() => {
		const requestId = focusRequestId;
		if (!active || !requestId) return;
		void tick().then(() =>
			requestAnimationFrame(() =>
				requestAnimationFrame(() => {
					if (!active || focusRequestId !== requestId) return;
					fit();
					shellStore.setFocusedPane('terminal');
					terminal?.focus();
				})
			)
		);
	});

	$effect(() => {
		const { fontSize } = terminalAppearance;
		const theme = terminalTheme;
		if (!terminal) return;
		terminal.options.fontFamily = DEFAULT_TERMINAL_FONT_FAMILY;
		terminal.options.fontSize = fontSize;
		terminal.options.theme = theme;
		scheduleFit();
	});

	onMount(() => {
		if (!host) return;
		const nextTerminal = new Terminal({
			allowProposedApi: false,
			convertEol: false,
			cursorBlink: true,
			fontFamily: DEFAULT_TERMINAL_FONT_FAMILY,
			fontSize: terminalAppearance.fontSize,
			macOptionClickForcesSelection: true,
			scrollback: 10_000,
			theme: terminalTheme
		});
		const nextFitAddon = new FitAddon();
		nextTerminal.loadAddon(nextFitAddon);
		nextTerminal.open(host);
		terminal = nextTerminal;
		fitAddon = nextFitAddon;
		applyAppearance(nextTerminal);
		const initialOutput = terminalStore.getSnapshot(sessionId)?.output;
		if (initialOutput) nextTerminal.write(initialOutput);
		const unsubscribe = terminalStore.subscribe(sessionId, (data) => nextTerminal.write(data));
		const inputDisposable = nextTerminal.onData(
			(data) => void terminalStore.write(sessionId, data)
		);
		const observer = new ResizeObserver(scheduleFit);
		observer.observe(host);
		scheduleFit();
		return () => {
			observer.disconnect();
			if (fitFrame !== null) cancelAnimationFrame(fitFrame);
			unsubscribe();
			inputDisposable.dispose();
			nextTerminal.dispose();
			terminal = null;
			fitAddon = null;
		};
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="terminal-instance"
	class:active
	style:background={terminalTheme.background}
	onfocusin={() => shellStore.setFocusedPane('terminal')}
	aria-hidden={!active}
>
	<div class="terminal-host">
		<div class="terminal-viewport" bind:this={host}></div>
	</div>
	{#if snapshot?.status === 'exited'}
		<div class="terminal-exited" aria-live="polite">Terminal exited</div>
	{/if}
</div>

<style>
	.terminal-instance {
		display: none;
		position: relative;
		flex: 1;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
	}

	.terminal-instance.active {
		display: block;
	}

	.terminal-host {
		position: relative;
		min-width: 0;
		min-height: 0;
		width: 100%;
		height: 100%;
		box-sizing: border-box;
	}

	.terminal-viewport {
		position: absolute;
		top: 10px;
		/* Side insets protect the status line ends while the footer reaches the panel bottom. */
		right: calc(var(--radius-window) + 6px);
		bottom: 0;
		left: calc(var(--radius-window) + 6px);
		min-width: 0;
		min-height: 0;
	}

	.terminal-viewport :global(.xterm) {
		height: 100%;
	}

	.terminal-exited {
		position: absolute;
		left: 12px;
		bottom: 12px;
		border-radius: 8px;
		padding: 7px 10px;
		background: rgba(15, 23, 42, 0.82);
		color: white;
		font-size: 12px;
	}
</style>
