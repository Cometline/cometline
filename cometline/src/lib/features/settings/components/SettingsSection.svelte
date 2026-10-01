<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		description = '',
		children
	}: {
		title: string;
		description?: string;
		children: Snippet;
	} = $props();
</script>

<section class="settings-section">
	<div class="settings-section-heading">
		<div>
			<h3>{title}</h3>
			{#if description}
				<p>{description}</p>
			{/if}
		</div>
	</div>
	<div class="settings-fields">
		{@render children()}
	</div>
</section>
