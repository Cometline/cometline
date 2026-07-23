<script lang="ts">
	import { FileText, Image, Video, Volume2 } from '@lucide/svelte';
	import {
		INPUT_MODALITY_LABEL,
		INPUT_MODALITY_ORDER,
		type InputModality
	} from '$lib/model-modalities';

	let {
		modalities = null,
		known = null
	}: {
		modalities?: readonly string[] | null;
		known?: boolean | null;
	} = $props();

	const supported = $derived(new Set(modalities ?? []));
</script>

{#if known === true}
	<span class="capability-icons">
		{#each INPUT_MODALITY_ORDER as modality (modality)}
			{@const active = supported.has(modality)}
			{@const label = INPUT_MODALITY_LABEL[modality as InputModality]}
			<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
			<span
				class="capability-icon"
				class:active
				role="img"
				aria-label={label}
				tabindex="0"
			>
				{#if modality === 'text'}
					<span class="capability-letter">T</span>
				{:else if modality === 'image'}
					<Image size={10} stroke-width={1.8} />
				{:else if modality === 'video'}
					<Video size={10} stroke-width={1.8} />
				{:else if modality === 'audio'}
					<Volume2 size={10} stroke-width={1.8} />
				{:else}
					<FileText size={10} stroke-width={1.8} />
				{/if}
				<span class="capability-tip" aria-hidden="true">{label}</span>
			</span>
		{/each}
	</span>
{/if}

<style>
	.capability-icons {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		flex-shrink: 0;
	}

	.capability-icon {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 17px;
		height: 17px;
		border: 1px solid var(--border-soft);
		border-radius: 3px;
		color: var(--text-soft);
		opacity: 0.28;
		outline: none;
	}

	.capability-icon.active {
		opacity: 1;
		color: var(--text-main);
		border-color: color-mix(in srgb, var(--text-main) 28%, var(--border-soft));
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
	}

	.capability-tip {
		position: absolute;
		bottom: calc(100% + 4px);
		left: 50%;
		transform: translateX(-50%);
		padding: 2px 6px;
		border-radius: 4px;
		background: rgba(30, 30, 30, 0.92);
		color: #fff;
		font-size: 9px;
		font-weight: 650;
		letter-spacing: 0.04em;
		line-height: 1.2;
		text-transform: uppercase;
		white-space: nowrap;
		pointer-events: none;
		opacity: 0;
		visibility: hidden;
		z-index: 1000;
	}

	.capability-icon:hover .capability-tip,
	.capability-icon:focus-visible .capability-tip {
		opacity: 1;
		visibility: visible;
	}

	.capability-letter {
		font-size: 9px;
		font-weight: 700;
		line-height: 1;
		letter-spacing: 0;
	}

	.capability-icon :global(svg) {
		display: block;
	}
</style>
