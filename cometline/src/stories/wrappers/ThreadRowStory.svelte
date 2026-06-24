<script lang="ts">
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';

	let { variant = 'user' as 'user' | 'assistant' | 'event', continuationRow = false } = $props();
</script>

<ThreadRow {variant} {continuationRow}>
	<span>{variant} row content</span>
</ThreadRow>
