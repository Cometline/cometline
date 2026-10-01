<script lang="ts">
	let {
		label,
		description = '',
		checked = $bindable(false),
		disabled = false,
		onchange
	}: {
		label: string;
		description?: string;
		checked?: boolean;
		disabled?: boolean;
		onchange?: (checked: boolean) => void | Promise<void>;
	} = $props();

	async function toggle() {
		if (disabled) return;
		checked = !checked;
		await onchange?.(checked);
	}
</script>

<div class="toggle-row">
	<div class="toggle-copy">
		<span class="toggle-label">{label}</span>
		{#if description}
			<p class="toggle-description">{description}</p>
		{/if}
	</div>
	<button
		type="button"
		class="switch"
		class:on={checked}
		role="switch"
		aria-checked={checked}
		aria-label={label}
		{disabled}
		onclick={toggle}
	>
		<span></span>
	</button>
</div>

<style>
	.toggle-row {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
	}

	.toggle-copy {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}

	.toggle-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}

	.toggle-description {
		margin: 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.switch {
		flex-shrink: 0;
		width: 44px;
		height: 28px;
		border: none;
		border-radius: 999px;
		background: rgba(203, 213, 225, 0.72);
		padding: 3px;
		display: flex;
		align-items: center;
		justify-content: flex-start;
		cursor: pointer;
		transition:
			background 160ms ease,
			transform 80ms cubic-bezier(0.2, 0, 0, 1);
		-webkit-tap-highlight-color: transparent;
	}

	.switch:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.switch:not(:disabled):hover {
		filter: brightness(0.92);
	}

	.switch:not(:disabled):active {
		transform: scale(0.96);
	}

	.switch:focus-visible {
		outline: none;
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.22);
	}

	.switch span {
		width: 22px;
		height: 22px;
		border-radius: 999px;
		background: white;
		box-shadow: 0 1px 5px rgba(15, 23, 42, 0.16);
	}

	.switch.on {
		justify-content: flex-end;
		background: #7aa1aa;
	}
</style>
