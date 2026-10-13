<script lang="ts">
	import {
		pastedTextLabel,
		pastedTextTitle
	} from '#lib/features/composer/rich-composer-paste.js';

	let { text = '', label }: { text?: string; label?: string } = $props();

	let shown = $derived(label ?? pastedTextLabel(text));
	let title = $derived(label ?? pastedTextTitle(text));
</script>

<span class="pasted-text-chip" {title} role="img" aria-label={shown}>{shown}</span>

<style>
	.pasted-text-chip {
		display: inline-flex;
		align-items: center;
		max-width: 16rem;
		padding: 0.1em 0.45em;
		border: 1px solid rgba(255, 255, 255, 0.28);
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.14);
		color: inherit;
		font-size: 0.92em;
		font-weight: 650;
		line-height: 1.3;
		white-space: nowrap;
		user-select: none;
	}
</style>
