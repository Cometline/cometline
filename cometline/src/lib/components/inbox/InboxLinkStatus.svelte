<script lang="ts">
	import { onDestroy } from 'svelte';
	import type { InboxMessageResource } from '$lib/client/cometmind';
	import {
		resolveInboxLinkAvailability,
		type LinkAvailabilityMap
	} from '$lib/inbox/link-availability';

	let {
		messages,
		onAvailability
	}: {
		messages: InboxMessageResource[];
		onAvailability: (next: LinkAvailabilityMap) => void;
	} = $props();

	$effect(() => {
		const currentMessages = messages;
		const controller = new AbortController();
		void resolveInboxLinkAvailability(currentMessages, {
			signal: controller.signal
		}).then((result) => {
			if (controller.signal.aborted) return;
			onAvailability(result);
		});
		return () => {
			controller.abort();
		};
	});

	onDestroy(() => {
		onAvailability({});
	});
</script>
