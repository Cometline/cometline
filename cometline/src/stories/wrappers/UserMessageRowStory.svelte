<script lang="ts">
	import UserMessageRow from '$lib/components/chat/UserMessageRow.svelte';

	let {
		copied = false,
		text = 'Explain how thread folding works.'
	}: { copied?: boolean; text?: string } = $props();
</script>

<UserMessageRow
	item={{ id: 'user-1', type: 'user', text }}
	avatarSrc="/project_avatar_192.png"
	copiedId={copied ? 'user-1' : null}
	onCopyMessage={() => {}}
/>
