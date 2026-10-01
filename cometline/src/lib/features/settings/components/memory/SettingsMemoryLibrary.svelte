<script lang="ts">
	import { LoaderCircle, Trash2 } from '@lucide/svelte';
	import type { SettingsMemoryPanel } from '$lib/features/settings/settings-memory-panel.svelte';

	let { panel }: { panel: SettingsMemoryPanel } = $props();
</script>

{#if panel.s.settings}
	<div class="settings-section">
		<div class="settings-section-heading">
			<div>
				<h3>Compaction</h3>
				<p>Preview or run memory compaction to merge and prune stored memories.</p>
			</div>
			<div class="memory-total" aria-label={`${panel.s.fullMemories.length} total memories`}>
				<strong>{panel.s.fullMemories.length}</strong>
				<span>total memories</span>
			</div>
		</div>

		<div class="actions">
			<button
				class="secondary"
				onclick={panel.previewCompact}
				disabled={panel.s.previewing || panel.s.compacting}
			>
				{#if panel.s.previewing}<span class="spin"><LoaderCircle size={14} /></span>{/if}
				Preview compaction
			</button>
			<button class="secondary" onclick={panel.runCompact} disabled={panel.s.compacting}>
				{#if panel.s.compacting}<span class="spin"><LoaderCircle size={14} /></span>{/if}
				Run compaction
			</button>
		</div>

		{#if panel.s.compactionPreview}
			<div class="compaction-feedback" data-testid="compaction-preview">
				<strong>Preview</strong>
				<span>
					{panel.s.compactionPreview.to_forget.length} to forget · {panel.s
						.compactionPreview.to_merge.length}
					merge {panel.s.compactionPreview.to_merge.length === 1 ? 'cluster' : 'clusters'} ·
					{panel.s.compactionPreview.active} of {panel.s.compactionPreview.max_memories} active
				</span>
			</div>
		{/if}

		{#if panel.s.compactionResult}
			<div class="compaction-feedback success" data-testid="compaction-result">
				<strong>Compaction complete</strong>
				<span>
					{panel.s.compactionResult.before} → {panel.s.compactionResult.after} memories ·
					{Math.max(0, panel.s.compactionResult.before - panel.s.compactionResult.after)} removed
				</span>
			</div>
		{/if}

		{#if panel.s.compactionError}
			<p class="compaction-error">{panel.s.compactionError}</p>
		{/if}
	</div>

	<div class="settings-section">
		<div class="settings-section-heading">
			<div>
				<h3>Memories</h3>
				<p>Search, add, or remove individual memories stored for this workspace.</p>
			</div>
		</div>

		<div class="add-row">
			<div class="add-row-header">
				<span>Add memory</span>
			</div>
			<textarea
				bind:value={panel.s.newContent}
				rows="3"
				placeholder="Something the agent should remember…"
				aria-label="Memory content"
			></textarea>
			<div class="add-memory-controls">
				<label>
					<span>Kind</span>
					<select
						value={panel.s.newKind}
						onchange={(event) => panel.selectNewKind(event.currentTarget.value)}
					>
						<option value="fact">Fact</option>
						<option value="project">Project</option>
						<option value="preference">Preference</option>
					</select>
				</label>
				{#if panel.s.newKind === 'preference'}
					<label>
						<span>Application</span>
						<select
							value={panel.s.newApplicationPolicy}
							onchange={(event) =>
								panel.selectNewApplicationPolicy(
									event.currentTarget.value as 'always' | 'relevant'
								)}
						>
							<option value="relevant">When relevant</option>
							<option value="always">Always</option>
						</select>
					</label>
				{/if}
				<label>
					<span>Retention</span>
					<select
						bind:value={panel.s.newRetentionPolicy}
						disabled={panel.retentionLocked}
						aria-label="Retention"
					>
						<option value="decaying">Decaying</option>
						<option value="protected">Protected</option>
					</select>
					{#if panel.retentionLocked}
						<small>Always-applied preferences are protected.</small>
					{/if}
				</label>
				<div class="add-row-actions">
					<button type="button" class="secondary" onclick={panel.addMemory}
						>Add memory</button
					>
				</div>
			</div>
		</div>

		<div class="search-row">
			<input
				type="search"
				bind:value={panel.s.searchQuery}
				placeholder="Search memories…"
				spellcheck="false"
				aria-busy={panel.visibleSearching}
			/>
		</div>

		<div class="memory-list scrollbar-none">
			{#each panel.visibleMemories as memory (memory.id)}
				<article class="memory-card">
					<div>
						<div class="memory-card-heading">
							<strong>{memory.kind.replaceAll('_', ' ')}</strong>
							<span class="memory-badge">{memory.application_policy}</span>
							<span class="memory-badge">{memory.retention_policy}</span>
						</div>
						<p>{memory.content}</p>
						<small>
							weight {memory.effective_weight.toFixed(2)} · accessed {memory.access_count}
							times
						</small>
					</div>
					<button
						class="icon danger"
						aria-label="Delete memory"
						onclick={() => panel.removeMemory(memory.id)}
					>
						<Trash2 size={14} />
					</button>
				</article>
			{:else}
				<p class="muted">No memories yet.</p>
			{/each}
		</div>

		{#if panel.s.memoryStatus}
			<p class="status">{panel.s.memoryStatus}</p>
		{/if}
	</div>
{/if}

<style>
	.muted,
	.status {
		font-size: 12px;
		color: var(--text-muted);
	}
	.actions,
	.search-row {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}

	.search-row {
		margin-top: 6px;
		padding-top: 16px;
		border-top: 1px solid var(--border-soft);
	}

	.memory-total {
		display: flex;
		flex: 0 0 auto;
		align-items: baseline;
		gap: 5px;
		color: var(--text-muted);
	}

	.memory-total strong {
		font-size: 18px;
		color: var(--text-main);
	}

	.memory-total span {
		font-size: 11px;
	}

	.compaction-feedback {
		display: grid;
		gap: 3px;
		padding: 10px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(251, 251, 250, 0.72);
		font-size: 12px;
		color: var(--text-muted);
	}

	.compaction-feedback strong {
		color: var(--text-main);
	}

	.compaction-feedback.success {
		border-color: color-mix(in srgb, var(--status-success) 25%, var(--border-soft));
	}

	.compaction-error {
		margin: 0;
		font-size: 12px;
		color: var(--status-error);
	}

	.search-row input {
		width: 100%;
		min-width: 0;
	}

	.add-row {
		display: grid;
		gap: 6px;
	}

	.add-row-header {
		display: flex;
		align-items: center;
	}

	.add-row-header span {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.add-row textarea {
		width: 100%;
		resize: vertical;
	}

	.add-memory-controls {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr)) auto;
		gap: 8px;
	}

	.add-row-actions {
		display: flex;
		grid-column: -2 / -1;
		align-self: start;
		justify-content: flex-end;
		padding-top: 24px;
	}

	.memory-card-heading {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
		text-transform: capitalize;
	}

	.memory-badge {
		padding: 2px 6px;
		border: 1px solid var(--border-soft);
		border-radius: 999px;
		background: color-mix(in srgb, var(--accent) 5%, transparent);
		color: var(--text-muted);
		font-size: 9px;
		font-weight: 600;
		line-height: 1.2;
		text-transform: capitalize;
	}

	.memory-list {
		display: grid;
		gap: 8px;
		max-height: 280px;
		overflow: auto;
	}

	.memory-card {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		padding: 12px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(251, 251, 250, 0.72);
	}

	.memory-card p {
		margin: 6px 0;
		font-size: 13px;
		color: var(--text-main);
	}

	.memory-card small {
		font-size: 11px;
		color: var(--text-soft);
	}

	.icon {
		border: none;
		background: transparent;
		color: var(--text-muted);
	}

	.icon.danger:hover {
		color: var(--status-error);
	}

	@media (max-width: 780px) {
		.add-memory-controls {
			grid-template-columns: 1fr;
		}
	}
</style>
