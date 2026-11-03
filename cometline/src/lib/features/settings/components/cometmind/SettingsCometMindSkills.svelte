<script lang="ts">
	import SettingsToggle from '../SettingsToggle.svelte';
	import type { CometMindSettings } from '#lib/cometmind-settings.js';

	let { cometmind = $bindable() }: { cometmind: CometMindSettings } = $props();
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Skills</h3>
		<p>
			CometMind reads Agent Skills from <code>~/.cometmind/skills</code>, workspace
			<code>.agents/skills</code>/<code>.claude/skills</code>, OpenCode, and Claude Code skill
			folders.
		</p>
	</div>
	<SettingsToggle
		label="Enable skills"
		description="Expose a compact skill index to CometMind and allow read-only loading via load_skill."
		bind:checked={cometmind.skills.enabled}
	/>
	<SettingsToggle
		label="Synthesize skill drafts from completed jobs"
		description="After a job completes, an LLM proposes a reusable skill draft for review. Choose its model in Models → Skill synthesis."
		bind:checked={cometmind.skills.synthesisEnabled}
		disabled={!cometmind.skills.enabled}
	/>
	<p class="settings-field-hint">Browse, refresh, and sync skills from the Skills page.</p>
</div>
