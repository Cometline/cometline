<script lang="ts">
	import { Keyboard, RotateCcw } from '@lucide/svelte';
	import type { ShortcutAction, ShortcutBinding, KeyboardShortcuts } from '$lib/types';
	import {
		SHORTCUT_DEFINITIONS,
		formatShortcut,
		captureShortcut,
		isDefaultBinding
	} from '$lib/keyboard-shortcuts';

	let {
		shortcuts,
		onChange
	}: {
		shortcuts: KeyboardShortcuts;
		onChange: (action: ShortcutAction, binding: ShortcutBinding) => void;
	} = $props();

	let editingAction = $state<ShortcutAction | null>(null);

	$effect(() => {
		window.electronAPI?.setShortcutCaptureActive?.(Boolean(editingAction));
	});

	$effect(() => {
		if (!editingAction) return;
		function onKeydown(e: KeyboardEvent) {
			if (e.key === 'Escape') {
				e.preventDefault();
				editingAction = null;
				return;
			}
			const action = editingAction;
			if (!action) return;
			const binding = captureShortcut(e);
			if (!binding) return;
			e.preventDefault();
			e.stopPropagation();
			onChange(action, binding);
			editingAction = null;
		}
		window.addEventListener('keydown', onKeydown, true);
		return () => window.removeEventListener('keydown', onKeydown, true);
	});

	function reset(action: ShortcutAction) {
		const def = SHORTCUT_DEFINITIONS.find((d) => d.id === action);
		if (!def) return;
		onChange(action, { ...def.defaultBinding });
	}
</script>

<div class="shortcuts-panel">
	<div class="shortcuts-header">
		<Keyboard size={16} />
		<div>
			<h3>Keyboard shortcuts</h3>
			<p>Click a shortcut and press the new key combination. Changes apply immediately.</p>
		</div>
	</div>

	<div class="shortcuts-list">
		{#each SHORTCUT_DEFINITIONS as def (def.id)}
			{@const binding = shortcuts[def.id]}
			{@const isEditing = editingAction === def.id}
			<div class="shortcut-row" class:editing={isEditing}>
				<span class="shortcut-label">{def.label}</span>

				{#if isEditing}
					<div class="shortcut-capture">
						<span class="capture-hint">Press a key combination…</span>
						<button
							class="secondary"
							onclick={() => (editingAction = null)}
							type="button"
						>
							Cancel
						</button>
					</div>
				{:else}
					<div class="shortcut-display">
						<kbd>{formatShortcut(binding)}</kbd>
						<button
							class="secondary"
							onclick={() => (editingAction = def.id)}
							type="button"
						>
							Change
						</button>
						<button
							class="secondary icon-only"
							onclick={() => reset(def.id)}
							disabled={isDefaultBinding(def.id, binding)}
							aria-label={`Reset ${def.label} shortcut`}
							title="Reset to default"
							type="button"
						>
							<RotateCcw size={14} />
						</button>
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>

<style>
	.shortcuts-panel {
		padding: 4px;
	}

	.shortcuts-header {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		margin-bottom: 18px;
		color: var(--text-main);
	}

	.shortcuts-header h3,
	.shortcuts-header p {
		margin: 0;
	}

	.shortcuts-header h3 {
		font-size: 15px;
		font-weight: 700;
	}

	.shortcuts-header p {
		font-size: 12px;
		color: var(--text-muted);
		margin-top: 2px;
	}

	.shortcuts-list {
		display: grid;
		gap: 8px;
	}

	.shortcut-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 10px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.72);
	}

	.shortcut-row.editing {
		background: rgba(0, 102, 204, 0.06);
		border-color: rgba(0, 102, 204, 0.35);
	}

	.shortcut-label {
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.shortcut-display,
	.shortcut-capture {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.shortcut-capture {
		gap: 12px;
	}

	kbd {
		display: inline-flex;
		align-items: center;
		min-width: 72px;
		justify-content: center;
		padding: 5px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.9);
		font-family: inherit;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
		box-shadow: 0 1px 0 rgba(15, 23, 42, 0.05);
	}

	.capture-hint {
		font-size: 12px;
		color: var(--text-muted);
		font-style: italic;
	}

	button {
		border: none;
		border-radius: 9px;
		padding: 7px 10px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		color: var(--text-main);
		background: rgba(15, 23, 42, 0.04);
	}

	button:hover:not(:disabled) {
		background: rgba(15, 23, 42, 0.08);
	}

	button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.icon-only {
		padding: 7px;
		display: inline-grid;
		place-items: center;
	}

	@media (max-width: 780px) {
		.shortcut-row {
			flex-direction: column;
			align-items: flex-start;
			gap: 10px;
		}
	}
</style>
