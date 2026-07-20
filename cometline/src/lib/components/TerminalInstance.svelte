<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import '@xterm/xterm/css/xterm.css';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';

	const MAX_CONTEXT_CHARS = 50_000;

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
	let terminal: Terminal | null = null;
	let fitAddon: FitAddon | null = null;
	let fitFrame: number | null = null;
	let selection = $state<{ text: string; top: number; left: number } | null>(null);
	let selectionError = $state('');
	const snapshot = $derived(terminalStore.getSnapshot(sessionId));

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

	function onMouseUp(event: MouseEvent) {
		queueMicrotask(() => {
			const text = terminal?.getSelection().trim() ?? '';
			if (!text) {
				selection = null;
				selectionError = '';
				return;
			}
			if (text.length > MAX_CONTEXT_CHARS) {
				selection = null;
				selectionError = 'Selection is too large. Select up to 50,000 characters.';
				return;
			}
			selectionError = '';
			selection = { text, top: event.clientY + 10, left: event.clientX };
		});
	}

	function addSelectionToContext() {
		if (!selection) return;
		shellStore.addWebContextForActive({
			kind: 'terminal',
			title: 'Terminal selection',
			source: `terminal://${sessionId}`,
			content: selection.text
		});
		shellStore.requestComposerFocus();
		terminal?.clearSelection();
		selection = null;
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

	onMount(() => {
		if (!host) return;
		const nextTerminal = new Terminal({
			allowProposedApi: false,
			convertEol: false,
			cursorBlink: true,
			fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
			fontSize: 12,
			macOptionClickForcesSelection: true,
			scrollback: 10_000,
			theme: {
				background: '#171717',
				foreground: '#f4f4f5',
				cursor: '#f4f4f5',
				selectionBackground: 'rgba(147, 197, 253, 0.35)'
			}
		});
		const nextFitAddon = new FitAddon();
		nextTerminal.loadAddon(nextFitAddon);
		nextTerminal.open(host);
		terminal = nextTerminal;
		fitAddon = nextFitAddon;
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
	onfocusin={() => shellStore.setFocusedPane('terminal')}
	onmouseup={onMouseUp}
	aria-hidden={!active}
>
	<div class="terminal-host" bind:this={host}></div>
	{#if selection}
		<button
			type="button"
			class="selection-context"
			style:left="{selection.left}px"
			style:top="{selection.top}px"
			onclick={addSelectionToContext}
		>
			Add to context
		</button>
	{/if}
	{#if selectionError}
		<div class="selection-error" role="status">{selectionError}</div>
	{/if}
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
		background: #171717;
	}

	.terminal-instance.active {
		display: block;
	}

	.terminal-host {
		min-width: 0;
		width: 100%;
		height: 100%;
		padding: 10px;
		box-sizing: border-box;
	}

	.terminal-host :global(.xterm) {
		height: 100%;
	}

	.selection-context {
		position: fixed;
		z-index: 40;
		border: none;
		border-radius: 8px;
		padding: 7px 10px;
		background: var(--text-main);
		color: white;
		font-size: 12px;
		font-weight: 650;
		box-shadow: 0 8px 20px rgba(15, 23, 42, 0.2);
		cursor: pointer;
	}

	.selection-error,
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

	.selection-error {
		background: rgba(180, 35, 24, 0.92);
	}
</style>
