<script lang="ts">
	let {
		label,
		value,
		disabled,
		onInput
	}: {
		label: string;
		value: string;
		disabled: boolean;
		onInput: (value: string) => void;
	} = $props();
</script>

<label>
	<span>{label}</span>
	<div class="color-field">
		<input
			type="color"
			{value}
			{disabled}
			aria-label={label}
			oninput={(e) => onInput(e.currentTarget.value)}
		/>
		<input
			type="text"
			{value}
			{disabled}
			spellcheck="false"
			pattern="^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$"
			oninput={(e) => onInput(e.currentTarget.value)}
		/>
	</div>
</label>

<style>
	.color-field {
		display: flex;
		align-items: center;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.color-field {
		gap: 8px;
	}

	input[type='color'] {
		width: 42px;
		height: 38px;
		padding: 2px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.76);
		cursor: pointer;
	}

	input[type='text'] {
		flex: 1;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.76);
		padding: 10px 11px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		outline: none;
	}

	input[type='text']:focus,
	input[type='color']:focus {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
	}

	input:disabled {
		cursor: not-allowed;
		opacity: 0.56;
	}
</style>
