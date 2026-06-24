<script lang="ts">
	import ThinkingBlock from '$lib/components/chat/ThinkingBlock.svelte';

	let { expanded = false }: { expanded?: boolean } = $props();
</script>

<ThinkingBlock
	text="Let me break this down into smaller tasks before editing files."
	{expanded}
	onToggle={() => {}}
/>
