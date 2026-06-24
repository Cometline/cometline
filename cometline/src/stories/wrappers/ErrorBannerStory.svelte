<script lang="ts">
	import ErrorBanner from '$lib/components/ErrorBanner.svelte';

	let {
		message = 'Something went wrong.',
		dismissible = false,
		onDismiss = () => {}
	}: {
		message?: string;
		dismissible?: boolean;
		onDismiss?: () => void;
	} = $props();
</script>

<ErrorBanner {message} onDismiss={dismissible ? onDismiss : undefined} />
