<script lang="ts">
	import type { TerminalAppearanceSettings } from '$lib/types';
	import {
		normalizeTerminalFontSize,
		TERMINAL_THEME_PRESETS
	} from '$lib/features/workspace/terminal-appearance';

	let { terminal = $bindable() }: { terminal: TerminalAppearanceSettings } = $props();

	let terminalTheme = $derived(TERMINAL_THEME_PRESETS[terminal.theme]);

	function updateTerminal<K extends keyof TerminalAppearanceSettings>(
		key: K,
		value: TerminalAppearanceSettings[K]
	) {
		terminal = { ...terminal, [key]: value };
	}
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<div>
			<h3>Terminal</h3>
			<p>Choose the text size and color scheme for the built-in terminal.</p>
		</div>
	</div>

	<div class="terminal-settings-grid">
		<div class="terminal-fields">
			<div class="terminal-inline-fields">
				<label>
					<span>Text size</span>
					<input
						type="number"
						min="8"
						max="32"
						value={terminal.fontSize}
						oninput={(event) =>
							updateTerminal(
								'fontSize',
								normalizeTerminalFontSize(event.currentTarget.valueAsNumber)
							)}
					/>
				</label>

				<label>
					<span>Color scheme</span>
					<select
						value={terminal.theme}
						onchange={(event) =>
							updateTerminal(
								'theme',
								event.currentTarget.value as typeof terminal.theme
							)}
					>
						{#each Object.entries(TERMINAL_THEME_PRESETS) as [id, preset] (id)}
							<option value={id}>{preset.label}</option>
						{/each}
					</select>
				</label>
			</div>
		</div>

		<div
			class="terminal-preview"
			style:background={terminalTheme.colors.background}
			style:color={terminalTheme.colors.foreground}
			style:font-size={`${terminal.fontSize}px`}
		>
			<div class="terminal-preview-title">cometline %</div>
			<div class="terminal-preview-line">
				<span style:color={terminalTheme.colors.green}>$ git status</span>
				<span style:color={terminalTheme.colors.cyan}> ready</span>
			</div>
			<p role="status">Preview reflects the selected text size and color scheme.</p>
		</div>
	</div>
</div>

<style>
	.terminal-settings-grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(220px, 0.8fr);
		gap: 16px;
		align-items: start;
	}

	.terminal-fields,
	.terminal-inline-fields {
		display: grid;
		gap: 12px;
		align-content: start;
	}

	.terminal-inline-fields {
		grid-template-columns: repeat(2, minmax(0, 1fr));
		align-items: start;
	}

	.terminal-preview {
		display: grid;
		align-content: center;
		gap: 8px;
		min-height: 130px;
		padding: 18px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		overflow: hidden;
		box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.04);
	}

	.terminal-preview-title {
		font-weight: 700;
	}

	.terminal-preview-line {
		white-space: nowrap;
	}

	.terminal-preview p {
		margin: 0;
		opacity: 0.7;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
		font-size: 11px;
		line-height: 1.35;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	input[type='number'],
	select {
		width: 100%;
		height: 42px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.76);
		padding: 10px 11px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		outline: none;
	}

	input[type='number']:focus,
	select:focus {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
	}

	@media (max-width: 780px) {
		.terminal-settings-grid,
		.terminal-inline-fields {
			grid-template-columns: 1fr;
		}
	}
</style>
