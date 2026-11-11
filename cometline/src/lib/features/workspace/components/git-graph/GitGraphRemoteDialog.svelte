<script lang="ts">
	let {
		open,
		title,
		description,
		remotes,
		remote,
		onCancel,
		onConfirm,
		onRemoteChange
	}: {
		open: boolean;
		title: string;
		description: string;
		remotes: string[];
		remote: string;
		onCancel: () => void;
		onConfirm: () => void;
		onRemoteChange: (remote: string) => void;
	} = $props();
</script>

{#if open}
	<dialog open class="remote-dialog" aria-labelledby="remote-dialog-title">
		<h2 id="remote-dialog-title">{title}</h2>
		<p>{description}</p>
		<label class="field">
			<span>Remote</span>
			<select value={remote} onchange={(event) => onRemoteChange(event.currentTarget.value)}>
				{#each remotes as name (name)}
					<option value={name}>{name}</option>
				{/each}
			</select>
		</label>
		<div class="actions">
			<button type="button" class="btn muted" onclick={onCancel}>Cancel</button>
			<button type="button" class="btn accent" onclick={onConfirm}>Push</button>
		</div>
	</dialog>
{/if}

<style>
	.remote-dialog {
		position: fixed;
		top: 50%;
		left: 50%;
		margin: 0;
		transform: translate(-50%, -50%);
		width: min(360px, calc(100vw - 48px));
		padding: 18px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 12px;
		background: var(--panel-bg);
	}
	.remote-dialog::backdrop {
		background: rgba(17, 24, 39, 0.28);
	}
	h2 {
		margin: 0;
		font-size: 15px;
		font-weight: 650;
	}
	p {
		margin: 8px 0 12px;
		font-size: 12px;
		color: var(--text-muted);
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
	}
	select {
		padding: 8px 10px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 8px;
		background: var(--panel-bg);
		color: var(--text-main);
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 14px;
	}
	.btn {
		padding: 6px 12px;
		border-radius: 8px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		background: transparent;
		font-size: 12px;
		cursor: pointer;
	}
	.accent {
		background: var(--accent);
		border-color: var(--accent);
		color: white;
	}
</style>
