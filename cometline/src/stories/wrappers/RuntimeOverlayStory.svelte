<script lang="ts">
	import { onMount } from 'svelte';
	import RuntimeOverlay from '#lib/features/shell/components/RuntimeOverlay.svelte';
	import { connectionState } from '#lib/stores/runtime.svelte.js';

	let { mode = 'connecting' as 'connecting' }: { mode?: 'connecting' } = $props();

	onMount(() => {
		if (mode === 'connecting') {
			connectionState.reconnect();
		}
	});
</script>

<div style="position:relative;min-height:320px;background:var(--app-bg);">
	<RuntimeOverlay />
</div>
