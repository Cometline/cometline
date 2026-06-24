<script lang="ts">
	import UserMessageRow from '$lib/components/chat/UserMessageRow.svelte';

	let { copied = false }: { copied?: boolean } = $props();
</script>

<UserMessageRow
	item={{ id: 'user-1', type: 'user', text: 'Explain how thread folding works.' }}
	iconVariant="default"
	copiedId={copied ? 'user-1' : null}
	onCopyMessage={() => {}}
/>
