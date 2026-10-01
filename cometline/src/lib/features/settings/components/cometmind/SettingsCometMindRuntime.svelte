<script lang="ts">
	import SettingsToggle from '../SettingsToggle.svelte';
	import type { CometMindSettings } from '$lib/cometmind-settings';

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
		<h3>Generation</h3>
		<p>
			Image and video tools use these xAI Imagine models. Other catalog models stay hidden
			until an adapter exists. Sign in with Grok first.
		</p>
	</div>
	<label>
		<span>Image model</span>
		<select
			value={`${cometmind.generation.image.providerId}::${cometmind.generation.image.model}`}
			onchange={(event) => {
				const [providerId = 'xai', model = 'grok-imagine-image-2.0'] = String(
					(event.currentTarget as HTMLSelectElement).value
				).split('::');
				cometmind = {
					...cometmind,
					generation: {
						...cometmind.generation,
						image: { providerId, model }
					}
				};
			}}
		>
			<option value="xai::grok-imagine-image-2.0">xAI · grok-imagine-image-2.0</option>
			<option value="xai::grok-imagine-image">xAI · grok-imagine-image</option>
			<option value="xai::grok-imagine-image-quality">
				xAI · grok-imagine-image-quality
			</option>
		</select>
	</label>
	<label>
		<span>Video model</span>
		<select
			value={`${cometmind.generation.video.providerId}::${cometmind.generation.video.model}`}
			onchange={(event) => {
				const [providerId = 'xai', model = 'grok-imagine-video-1.5'] = String(
					(event.currentTarget as HTMLSelectElement).value
				).split('::');
				cometmind = {
					...cometmind,
					generation: {
						...cometmind.generation,
						video: { providerId, model }
					}
				};
			}}
		>
			<option value="xai::grok-imagine-video-1.5">xAI · grok-imagine-video-1.5</option>
			<option value="xai::grok-imagine-video">xAI · grok-imagine-video</option>
		</select>
		<p class="settings-field-hint">
			Sign in with Grok first. Other catalog image/video models stay hidden until an adapter
			exists.
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
