<script lang="ts">
	import type { Snippet } from 'svelte';

	let { label, children }: { label: string; children: Snippet } = $props();
</script>

<label class="field">
	<span class="field-label">{label}</span>
	{@render children()}
</label>

<style>
	.field {
		display: grid;
		gap: 6px;
		margin-bottom: 16px;
	}

	.field-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}
</style>
