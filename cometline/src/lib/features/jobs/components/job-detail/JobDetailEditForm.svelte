<script lang="ts">
	import WorkspacePathField from '#lib/components/WorkspacePathField.svelte';

	let {
		description = $bindable(''),
		dod = $bindable(''),
		workspacePath = $bindable(''),
		saving = false,
		onSave
	}: {
		description?: string;
		dod?: string;
		workspacePath?: string;
		saving?: boolean;
		onSave?: () => void | Promise<void>;
	} = $props();
</script>

<form
	class="drawer-form"
	onsubmit={(e) => {
		e.preventDefault();
		void onSave?.();
	}}
>
	<div class="settings-field">
		<label>
			<span>Description</span>
			<textarea bind:value={description} rows={3}></textarea>
		</label>
	</div>
	<div class="settings-field">
		<label>
			<span>Definition of done</span>
			<textarea bind:value={dod} rows={3}></textarea>
		</label>
	</div>
	<div class="settings-field">
		<span class="field-label">Workspace path</span>
		<WorkspacePathField bind:value={workspacePath} />
	</div>
	<button type="submit" class="secondary" disabled={saving}>Save changes</button>
</form>

<style>
	.drawer-form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.drawer-form textarea {
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 8px 10px;
		font: inherit;
		font-size: 12px;
		background: var(--app-bg);
		color: var(--text-main);
		resize: vertical;
	}

	.field-label {
		display: block;
		margin-bottom: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}
</style>
