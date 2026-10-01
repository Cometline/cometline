<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		variant = 'secondary',
		type = 'button',
		disabled = false,
		class: className = '',
		onclick,
		children
	}: {
		variant?: 'secondary' | 'primary' | 'danger';
		type?: 'button' | 'submit';
		disabled?: boolean;
		class?: string;
		onclick?: (event: MouseEvent) => void | Promise<void>;
		children: Snippet;
	} = $props();
</script>

<button {type} class="btn {variant} {className}" {disabled} {onclick}>
	{@render children()}
</button>

<style>
	.btn {
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		cursor: pointer;
		transition:
			background 140ms ease,
			border-color 140ms ease,
			color 140ms ease,
			box-shadow 140ms ease,
			transform 80ms cubic-bezier(0.2, 0, 0, 1);
		-webkit-tap-highlight-color: transparent;
	}

	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
		transform: none;
		box-shadow: none;
	}

	.btn:focus-visible {
		outline: none;
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.22);
	}

	.secondary {
		background: rgba(255, 255, 255, 0.82);
		color: var(--text-main);
	}

	.secondary:hover:not(:disabled) {
		background: rgba(15, 23, 42, 0.08);
		border-color: rgba(15, 23, 42, 0.18);
		box-shadow: none;
	}

	.secondary:active:not(:disabled) {
		transform: scale(0.97);
		background: rgba(15, 23, 42, 0.12);
	}

	.danger {
		background: rgba(255, 255, 255, 0.82);
		color: var(--text-main);
	}

	.danger:hover:not(:disabled) {
		background: rgba(180, 35, 24, 0.08);
		border-color: rgba(180, 35, 24, 0.22);
		color: var(--status-error);
	}

	.danger:active:not(:disabled) {
		transform: scale(0.97);
	}

	.primary {
		background: var(--text-main);
		color: white;
		border-color: transparent;
	}

	.primary:hover:not(:disabled) {
		background: #0f172a;
		box-shadow: none;
	}

	.primary:active:not(:disabled) {
		transform: scale(0.97);
		background: #020617;
	}
</style>
