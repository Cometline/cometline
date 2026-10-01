<script lang="ts">
	import { RotateCcw, Search } from '@lucide/svelte';
	import type { ShortcutAction, ShortcutBinding, KeyboardShortcuts } from '$lib/types';
	import {
		SHORTCUT_DEFINITIONS,
		shortcutsByCategory,
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
	let filterQuery = $state('');

	const groupedShortcuts = shortcutsByCategory();
	const shortcutCount = SHORTCUT_DEFINITIONS.length;

	const filteredGroups = $derived.by(() => {
		const query = filterQuery.trim().toLowerCase();
		if (!query) return groupedShortcuts;
		return groupedShortcuts
			.map((group) => ({
				...group,
				shortcuts: group.shortcuts.filter((def) => {
					const binding = shortcuts[def.id];
					const haystack = [
						def.label,
						def.id,
						group.category.title,
						group.category.description,
						formatShortcut(binding)
					]
						.join(' ')
						.toLowerCase();
					return haystack.includes(query);
				})
			}))
			.filter((group) => group.shortcuts.length > 0);
	});

	const matchCount = $derived(
		filteredGroups.reduce((count, group) => count + group.shortcuts.length, 0)
	);

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
		const def = SHORTCUT_DEFINITIONS.find((entry) => entry.id === action);
		if (!def) return;
		onChange(action, { ...def.defaultBinding });
	}
</script>

<div class="shortcuts-panel settings-panel-frame">
	<div class="settings-panel-body">
		<div class="shortcuts-filter">
			<label class="shortcuts-search">
				<Search size={14} aria-hidden="true" />
				<input
					type="search"
					bind:value={filterQuery}
					placeholder="Filter shortcuts…"
					spellcheck="false"
					autocomplete="off"
					aria-label="Filter shortcuts"
				/>
			</label>
			<span class="shortcuts-filter-count">
				{#if matchCount === shortcutCount}
					{shortcutCount}
				{:else}
					{matchCount} / {shortcutCount}
				{/if}
			</span>
		</div>

		{#if filteredGroups.length === 0}
			<p class="shortcuts-empty">No shortcuts match “{filterQuery.trim()}”.</p>
		{/if}

		{#each filteredGroups as group (group.category.id)}
			<section class="settings-section">
				<div class="settings-section-heading">
					<div>
						<h3>{group.category.title}</h3>
						<p>{group.category.description}</p>
					</div>
				</div>

				<div class="shortcuts-list">
					{#each group.shortcuts as def (def.id)}
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
			</section>
		{/each}
	</div>
</div>

<style>
	.shortcuts-filter {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 16px;
	}

	.shortcuts-search {
		display: flex;
		flex: 1;
		align-items: center;
		gap: 8px;
		min-width: 0;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 8px 10px;
		background: rgba(255, 255, 255, 0.82);
		color: var(--text-muted);
	}

	.shortcuts-search:focus-within {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
		color: var(--text-main);
	}

	.shortcuts-search input {
		flex: 1;
		min-width: 0;
		border: 0;
		background: transparent;
		padding: 0;
		font-size: 12px;
		color: var(--text-main);
		outline: none;
		box-shadow: none;
	}

	.shortcuts-search input::-webkit-search-decoration,
	.shortcuts-search input::-webkit-search-cancel-button,
	.shortcuts-search input::-webkit-search-results-button {
		display: none;
	}

	.shortcuts-search input::placeholder {
		color: var(--text-muted);
	}

	.shortcuts-filter-count {
		flex-shrink: 0;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-muted);
	}

	.shortcuts-empty {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.shortcuts-list {
		display: grid;
		gap: 0;
	}

	.shortcut-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 10px 0;
		border: none;
		border-radius: 0;
		background: transparent;
	}

	.shortcut-row + .shortcut-row {
		border-top: 1px solid var(--border-soft);
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

	.icon-only {
		padding: 8px;
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
