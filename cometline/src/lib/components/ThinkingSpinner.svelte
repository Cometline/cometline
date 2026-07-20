<script lang="ts">
	import ThinkingIndicator from './ThinkingIndicator.svelte';
	import type { ThinkingIndicatorVariant } from './thinking-indicator';

	/** Inline-sized comet indicator (alias for ThinkingIndicator). */
	let {
		size = 12,
		color,
		label = 'Thinking',
		variant = 'orbit'
	}: {
		size?: number;
		color?: string;
		label?: string;
		variant?: ThinkingIndicatorVariant;
	} = $props();
</script>

<ThinkingIndicator {size} {color} {label} {variant} />
