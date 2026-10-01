<script lang="ts">
	import ThreadRow from './ThreadRow.svelte';
</script>

<ThreadRow variant="user" continuationRow={true}>
	<span data-testid="content">Hello</span>
</ThreadRow>
