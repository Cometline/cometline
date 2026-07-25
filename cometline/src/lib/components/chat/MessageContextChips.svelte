<script lang="ts">
	import { FileText, Globe, Terminal } from '@lucide/svelte';
	import type { MessageContextRef } from '$lib/types';
	import {
		messageContextLabel,
		openMessageContext
	} from '$lib/chat/message-context';

	let {
		contexts,
		interactive = true,
		removable = false,
		onRemove,
		onClearAll,
		align = 'start'
	}: {
		contexts: MessageContextRef[];
		/** When true, chips navigate on click. */
		interactive?: boolean;
		/** Show per-chip remove buttons (composer pending chips). */
		removable?: boolean;
		onRemove?: (index: number) => void;
		onClearAll?: () => void;
		align?: 'start' | 'end';
	} = $props();

	function iconFor(kind: MessageContextRef['kind']) {
		if (kind === 'page') return Globe;
		if (kind === 'terminal') return Terminal;
		return FileText;
	}
</script>

{#if contexts.length > 0}
	<div
		class="message-context-chips"
		class:align-end={align === 'end'}
		role="list"
		aria-label="Chat context"
	>
		{#each contexts as context, index (context.source + ':' + index)}
			{@const Icon = iconFor(context.kind)}
			{@const label = messageContextLabel(context)}
			<div class="message-context-chip-wrap" role="listitem">
				{#if interactive && !removable}
					<button
						type="button"
						class="message-context-chip interactive"
						title={label}
						onclick={() => openMessageContext(context)}
					>
						<Icon size={14} />
						<span>{label}</span>
					</button>
				{:else}
					<div class="message-context-chip">
						{#if interactive}
							<button
								type="button"
								class="message-context-chip-open"
								title={label}
								onclick={() => openMessageContext(context)}
							>
								<Icon size={14} />
								<span>{label}</span>
							</button>
						{:else}
							<Icon size={14} />
							<span title={label}>{label}</span>
						{/if}
						{#if removable && onRemove}
							<button
								type="button"
								class="message-context-chip-remove"
								onclick={() => onRemove(index)}
								aria-label="Remove {label}"
							>
								×
							</button>
						{/if}
					</div>
				{/if}
			</div>
		{/each}
		{#if removable && contexts.length > 1 && onClearAll}
			<button type="button" class="message-context-clear" onclick={onClearAll}>
				Clear all
			</button>
		{/if}
	</div>
{/if}

<style>
	.message-context-chips {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px;
	}

	.message-context-chips.align-end {
		justify-content: flex-end;
	}

	.message-context-chip-wrap {
		display: contents;
	}

	.message-context-chip,
	.message-context-chip.interactive {
		display: flex;
		align-items: center;
		gap: 7px;
		max-width: 100%;
		padding: 7px 9px;
		border: 1px solid
			color-mix(in srgb, var(--hero-composer-glow-color, #72c0ff) 20%, var(--border-soft));
		border-radius: 9px;
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 7%,
			var(--panel-bg)
		);
		color: var(--text-muted);
		font-size: 12px;
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.58);
	}

	.message-context-chip.interactive {
		cursor: pointer;
		font: inherit;
		text-align: left;
	}

	.message-context-chip.interactive:hover,
	.message-context-chip-open:hover {
		color: var(--text-main);
		border-color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 36%,
			var(--border-soft)
		);
	}

	.message-context-chip-open {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		min-width: 0;
		padding: 0;
		border: 0;
		background: transparent;
		color: inherit;
		font: inherit;
		cursor: pointer;
		text-align: left;
	}

	.message-context-clear {
		border: 0;
		background: transparent;
		color: var(--text-soft);
		font-size: 12px;
		cursor: pointer;
		padding: 4px 6px;
	}

	.message-context-clear:hover {
		color: var(--text-main);
	}

	.message-context-chip :global(svg) {
		flex-shrink: 0;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 62%,
			var(--accent, #0066cc)
		);
	}

	.message-context-chip span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.message-context-chip-remove {
		margin-left: auto;
		border: 0;
		background: transparent;
		color: var(--text-soft);
		font-size: 18px;
		line-height: 1;
		cursor: pointer;
	}

	.message-context-chip-remove:hover {
		color: var(--text-main);
	}
</style>
