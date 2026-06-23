<script lang="ts">
	import { fly } from 'svelte/transition';
	import type { Snippet } from 'svelte';

	let {
		ariaLabel,
		class: className = '',
		menuRef = $bindable<HTMLDivElement | null>(null),
		children
	}: {
		ariaLabel: string;
		class?: string;
		menuRef?: HTMLDivElement | null;
		children: Snippet;
	} = $props();
</script>

<div
	class="skill-command-menu scrollbar-none {className}"
	role="listbox"
	aria-label={ariaLabel}
	bind:this={menuRef}
	transition:fly={{ y: 6, duration: 120 }}
>
	{@render children()}
</div>

<style>
	.skill-command-menu {
		position: absolute;
		left: 14px;
		right: 14px;
		bottom: calc(100% + 8px);
		z-index: 28;
		max-height: 260px;
		overflow: auto;
		padding: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(246, 249, 252, 0.98);
		box-shadow: var(--shadow-card);
	}

	.skill-command-menu :global(.skill-command-option) {
		display: flex;
		width: 100%;
		flex-direction: column;
		gap: 3px;
		padding: 9px 10px;
		border: none;
		border-radius: 10px;
		background: transparent;
		text-align: left;
		cursor: pointer;
	}

	.skill-command-menu :global(.skill-command-option:hover),
	.skill-command-menu :global(.skill-command-option.highlighted) {
		background: rgba(15, 23, 42, 0.06);
	}

	.skill-command-menu :global(.skill-command-name) {
		font-size: 12px;
		font-weight: 700;
		color: var(--text-main);
	}

	.skill-command-menu :global(.skill-command-description) {
		font-size: 11px;
		line-height: 1.35;
		color: var(--text-soft);
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		overflow: hidden;
	}

	.skill-command-menu :global(.skill-command-empty) {
		margin: 0;
		padding: 10px 12px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.skill-command-menu :global(.skill-command-loading) {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 0;
		padding: 10px 12px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.skill-command-menu :global(.slash-group-heading) {
		margin: 0;
		padding: 8px 10px 4px;
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--text-soft);
	}

	.skill-command-menu :global(.slash-group-heading:first-child) {
		padding-top: 4px;
	}

	.skill-command-menu :global(.workspace-search-hint) {
		display: flex;
		align-items: center;
		gap: 7px;
		margin: 2px 2px 6px;
		padding: 7px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 9px;
		background: rgba(255, 255, 255, 0.7);
		color: var(--text-soft);
		font-size: 12px;
		line-height: 1.2;
	}

	.skill-command-menu :global(.workspace-search-hint svg) {
		flex-shrink: 0;
		color: var(--text-soft);
	}

	.skill-command-menu :global(.workspace-search-value) {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-main);
		font-weight: 500;
	}

	.skill-command-menu :global(.workspace-search-placeholder) {
		color: var(--text-soft);
	}

	.skill-command-menu :global(.workspace-option-row) {
		display: flex;
		align-items: stretch;
		gap: 4px;
		border-radius: 10px;
	}

	.skill-command-menu :global(.workspace-option-row.highlighted),
	.skill-command-menu :global(.workspace-option-row:hover),
	.skill-command-menu :global(.workspace-option-row:focus-within) {
		background: rgba(15, 23, 42, 0.06);
	}

	.skill-command-menu :global(.workspace-option-row .skill-command-option) {
		flex: 1;
		min-width: 0;
	}

	.skill-command-menu :global(.workspace-option-row .skill-command-option:hover),
	.skill-command-menu :global(.workspace-option-row .skill-command-option.highlighted) {
		background: transparent;
	}

	.skill-command-menu :global(.workspace-delete-btn) {
		flex: 0 0 auto;
		align-self: center;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		margin-right: 4px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-soft);
		cursor: pointer;
		opacity: 0;
		transition: opacity 120ms ease;
	}

	.skill-command-menu :global(.workspace-option-row:hover .workspace-delete-btn),
	.skill-command-menu :global(.workspace-option-row:focus-within .workspace-delete-btn),
	.skill-command-menu :global(.workspace-delete-btn:focus-visible) {
		opacity: 1;
	}

	.skill-command-menu :global(.workspace-delete-btn:hover) {
		background: rgba(180, 35, 24, 0.1);
		color: #b42318;
	}

	.skill-command-menu :global(.workspace-delete-btn:disabled) {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.skill-command-menu :global(.skill-command-loading .mention-spinner) {
		flex-shrink: 0;
		color: var(--text-soft);
		animation: mention-spin 0.7s linear infinite;
	}

	@keyframes mention-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.skill-command-menu :global(.skill-command-loading .mention-spinner) {
			animation: none;
		}
	}

	.skill-command-menu :global(.mention-option) {
		flex-direction: row;
		align-items: center;
		gap: 8px;
		padding: 7px 10px;
	}

	.skill-command-menu :global(.mention-option svg) {
		flex-shrink: 0;
		color: var(--text-soft);
	}

	.skill-command-menu :global(.mention-path) {
		font-size: 12px;
		font-weight: 500;
		color: var(--text-main);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.skill-command-menu :global(.mention-hint) {
		margin: 2px 0 0;
		padding: 6px 10px 2px;
		font-size: 10px;
		color: var(--text-soft);
		border-top: 1px solid var(--border-soft);
	}

	.skill-command-menu:global(.model-command-menu) {
		max-height: 320px;
		overflow-y: auto;
	}

	.skill-command-menu :global(.model-command-option) {
		position: relative;
	}

	.skill-command-menu :global(.model-command-option.is-selected) {
		background: rgba(0, 102, 204, 0.04);
	}

	.skill-command-menu :global(.model-command-check) {
		position: absolute;
		right: 10px;
		top: 50%;
		transform: translateY(-50%);
		color: rgba(0, 102, 204, 0.7);
	}
</style>
