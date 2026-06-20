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
		border: none;
		border-radius: 10px;
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		cursor: pointer;
	}

	.btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.secondary {
		background: rgba(15, 23, 42, 0.04);
		color: var(--text-main);
	}

	.secondary:hover:not(:disabled) {
		background: rgba(15, 23, 42, 0.08);
	}

	.danger:hover:not(:disabled) {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.primary {
		background: var(--text-main);
		color: white;
	}

	.primary:hover:not(:disabled) {
		background: #111827;
	}
</style>
