<script lang="ts">
	import { onMount } from 'svelte';
	import RuntimeOverlay from '$lib/components/RuntimeOverlay.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';

	let { mode = 'connecting' as 'connecting' | 'error' }: { mode?: 'connecting' | 'error' } = $props();

	onMount(async () => {
		if (mode === 'connecting') {
			connectionState.reconnect();
			return;
		}
		globalThis.fetch = async () => {
			throw new Error('Connection refused');
		};
		await connectionState.check();
	});
</script>

<div style="position:relative;min-height:320px;background:var(--app-bg);">
	<RuntimeOverlay />
</div>
