<script lang="ts">
	import WorkspacePathField from '$lib/components/WorkspacePathField.svelte';

	let {
		description = $bindable(''),
		dod = $bindable(''),
		workspacePath = $bindable(''),
		saving = false,
		submitLabel = 'Create',
		onSubmit
	}: {
		description?: string;
		dod?: string;
		workspacePath?: string;
		saving?: boolean;
		submitLabel?: string;
		onSubmit: () => void | Promise<void>;
	} = $props();
</script>

<form
	class="job-create-form settings-ui"
	onsubmit={(e) => {
		e.preventDefault();
		void onSubmit();
	}}
>
	<div class="settings-field">
		<label>
			<span>Description</span>
			<textarea
				bind:value={description}
				rows={3}
				required
				placeholder="What needs to be done?"
			></textarea>
		</label>
	</div>
	<div class="settings-field">
		<label>
			<span>Definition of done</span>
			<textarea bind:value={dod} rows={3} placeholder="How will you know it's finished?"
			></textarea>
		</label>
	</div>
	<div class="settings-field">
		<span class="field-label">Workspace path (optional)</span>
		<WorkspacePathField bind:value={workspacePath} />
	</div>
	<div class="job-create-actions">
		<button type="submit" class="primary" disabled={saving || !description.trim()}>
			{submitLabel}
		</button>
	</div>
</form>

<style>
	.job-create-form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.job-create-form textarea {
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

	.job-create-form textarea:focus {
		outline: none;
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
	}

	.job-create-actions {
		display: flex;
		justify-content: flex-end;
	}

	.field-label {
		display: block;
		margin-bottom: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}
</style>
