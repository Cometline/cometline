<script lang="ts">
	import SettingsToggle from '../SettingsToggle.svelte';
	import type { CometMindSettings } from '#lib/cometmind-settings.js';

	let { cometmind = $bindable() }: { cometmind: CometMindSettings } = $props();
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Runtime</h3>
		<p>
			Controls sent to CometMind for each agent response. Settings are saved to
			<code>~/.cometmind/cometline-settings.json</code>.
		</p>
	</div>
	<label>
		<span>Log level</span>
		<select bind:value={cometmind.logLevel}>
			<option value="error">Error</option>
			<option value="warn">Warn</option>
			<option value="info">Info</option>
			<option value="debug">Debug</option>
		</select>
		<p class="settings-field-hint">
			Controls what CometMind writes to <code>~/.cometmind/logs/cometline.log</code>
			and
			<code>cometline-gateway.log</code>. Applied on Save by restarting CometMind.
		</p>
	</label>
</div>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Coding task delegation</h3>
		<p>
			Optional external harness for <code>delegate_coding_task</code>. Native
			<code>edit_file</code> / <code>run_command</code> work without this. Settings are
			written under <code>cometmind.acp</code> in
			<code>~/.cometmind/cometline-settings.json</code>.
		</p>
	</div>
	<SettingsToggle
		label="Enable external coding harness"
		description="Off by default. When enabled and the selected CLI is installed, CometMind can delegate multi-file coding via delegate_coding_task."
		bind:checked={cometmind.acp.enabled}
	/>
	<label>
		<span>Coding harness</span>
		<select bind:value={cometmind.acp.defaultHarness} disabled={!cometmind.acp.enabled}>
			<option value="opencode">OpenCode</option>
			<option value="claude">Claude Code</option>
			<option value="codex">Codex</option>
		</select>
		<p class="settings-field-hint">
			Each harness uses a built-in non-interactive CLI profile. Claude Code and Codex use
			their own local login/configuration. Only select a harness for workspaces you trust.
		</p>
	</label>
</div>
