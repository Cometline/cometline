<script lang="ts">
	import ThinkingBlock from '#lib/features/chat/components/ThinkingBlock.svelte';

	let { expanded = false }: { expanded?: boolean } = $props();
</script>

<ThinkingBlock
	text="Let me break this down into smaller tasks before editing files."
	{expanded}
	onToggle={() => {}}
/>
