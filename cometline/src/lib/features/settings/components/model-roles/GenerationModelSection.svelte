<script lang="ts">
	import type { CometMindSettings } from '#lib/cometmind-settings.js';

	let { cometmind = $bindable() }: { cometmind: CometMindSettings } = $props();
</script>

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
					event.currentTarget.value
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
					event.currentTarget.value
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
