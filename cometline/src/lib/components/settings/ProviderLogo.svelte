<script lang="ts">
	import anthropicIcon from '$lib/assets/provider-icons/anthropic.svg';
	import grokIcon from '$lib/assets/provider-icons/grok.svg';
	import openaiIcon from '$lib/assets/provider-icons/openai.svg';
	import opencodeIcon from '$lib/assets/provider-icons/opencode.svg';
	import type { ProviderMethod } from '$lib/types';

	const BRAND_ICONS: Partial<Record<ProviderMethod, string>> = {
		openai: openaiIcon,
		codex: openaiIcon,
		anthropic: anthropicIcon,
		xai: grokIcon,
		'opencode-go': opencodeIcon
	};

	let { method }: { method: ProviderMethod } = $props();
	let icon = $derived(BRAND_ICONS[method]);
</script>

<span class="provider-logo" aria-hidden="true">
	{#if icon}
		<img src={icon} alt="" />
	{:else}
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-linecap="round"
			stroke-linejoin="round"
			stroke-width="2.2"
		>
			<path d="m9 7-5 5 5 5M15 7l5 5-5 5" />
			<path d="m13 4-2 16" />
		</svg>
	{/if}
</span>

<style>
	.provider-logo {
		display: grid;
		flex: 0 0 auto;
		place-items: center;
		width: 25px;
		height: 25px;
		border-radius: 7px;
		color: var(--text-main);
		background: #eef2f7;
	}

	.provider-logo :global(img),
	.provider-logo svg {
		width: 16px;
		height: 16px;
	}
</style>
