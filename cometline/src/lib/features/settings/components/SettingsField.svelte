<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label,
		note,
		children
	}: {
		label: string;
		note?: string;
		children: Snippet;
	} = $props();
</script>

<label class="field">
	<span class="field-label">{label}</span>
	{@render children()}
	{#if note}
		<span class="field-hint">{note}</span>
	{/if}
</label>

<style>
	.field {
		display: grid;
		gap: 6px;
	}

	.field-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.field-hint {
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-soft);
	}
</style>
