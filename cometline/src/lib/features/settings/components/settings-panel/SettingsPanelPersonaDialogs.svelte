<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import type { createSettingsPanelPersonaEditor } from '../../settings-panel-persona-editor.svelte';

	let { editor }: { editor: ReturnType<typeof createSettingsPanelPersonaEditor> } = $props();

	let personaAvatarFileInput = $state<HTMLInputElement | undefined>(undefined);
</script>

{#if editor.soulPreviewOpen}
	<div class="soul-preview-overlay" transition:fade={{ duration: 100 }}>
		<button
			class="soul-preview-scrim"
			aria-label="Close SOUL preview"
			onclick={editor.closeSoulPreview}
		></button>
		<div
			class="soul-preview-panel"
			role="dialog"
			aria-modal="true"
			aria-label="SOUL preview"
			transition:fly={{ y: 8, duration: 140 }}
		>
			<div class="soul-preview-header">
				<h4>{editor.soulPreviewLabel} — SOUL.md</h4>
				<div class="soul-preview-header-actions">
					<button type="button" class="secondary" onclick={editor.copySoulPreview}>
						{editor.soulPreviewCopied ? 'Copied' : 'Copy'}
					</button>
					<button type="button" class="secondary" onclick={editor.closeSoulPreview}
						>Close</button
					>
				</div>
			</div>
			<pre class="soul-preview-body">{editor.soulPreviewText}</pre>
		</div>
	</div>
{/if}

{#if editor.personaEditorOpen}
	<div class="soul-preview-overlay" transition:fade={{ duration: 100 }}>
		<button
			class="soul-preview-scrim"
			aria-label="Close persona editor"
			onclick={editor.closePersonaEditor}
		></button>
		<div
			class="soul-preview-panel persona-editor-panel settings-panel-frame"
			role="dialog"
			aria-modal="true"
			aria-label="Persona editor"
			transition:fly={{ y: 8, duration: 140 }}
		>
			<div class="soul-preview-header">
				<h4>{editor.personaEditorId ? 'Edit persona' : 'New persona'}</h4>
				<button type="button" class="secondary" onclick={editor.closePersonaEditor}
					>Close</button
				>
			</div>
			{#if editor.personaEditorError}
				<p class="persona-editor-error">{editor.personaEditorError}</p>
			{/if}
			<div class="settings-fields">
				<label class="settings-field">
					<span>Name</span>
					<input type="text" bind:value={editor.personaEditorName} />
				</label>
				<label class="settings-field">
					<span>Avatar image</span>
					<div class="persona-editor-avatar-row">
						{#if editor.personaEditorAvatarDataUrl}
							<img
								class="persona-editor-avatar-preview"
								src={editor.personaEditorAvatarDataUrl}
								alt=""
								width="64"
								height="64"
							/>
						{/if}
						<button
							type="button"
							class="secondary"
							onclick={() => personaAvatarFileInput?.click()}
						>
							Choose image
						</button>
						<input
							bind:this={personaAvatarFileInput}
							type="file"
							class="persona-editor-file-input"
							accept="image/png,image/jpeg,image/webp"
							onchange={editor.onPersonaAvatarFileChange}
						/>
					</div>
				</label>
				<label class="settings-field">
					<span>SOUL.md content</span>
					<textarea rows="10" bind:value={editor.personaEditorSoul}></textarea>
				</label>
			</div>
			<div class="persona-editor-actions">
				<button
					type="button"
					class="primary"
					disabled={editor.personaEditorBusy}
					onclick={editor.submitPersonaEditor}
				>
					{editor.personaEditorBusy ? 'Saving…' : 'Save persona'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.soul-preview-overlay {
		position: fixed;
		inset: 0;
		z-index: 100;
		display: grid;
		place-items: center;
		padding: 24px;
	}

	.soul-preview-scrim {
		position: absolute;
		inset: 0;
		border: none;
		background: rgba(15, 23, 42, 0.18);
		cursor: pointer;
	}

	.soul-preview-panel {
		position: relative;
		z-index: 1;
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		gap: 12px;
		width: min(560px, 100%);
		max-height: min(640px, calc(100vh - 60px));
		overflow: hidden;
		padding: 16px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.98);
		box-shadow: var(--shadow-card);
	}

	.persona-editor-panel {
		background: rgba(255, 255, 255, 0.98);
		grid-template-rows: auto auto minmax(0, auto) auto;
		overflow-y: auto;
	}

	.soul-preview-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
	}

	.soul-preview-header h4 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
		color: var(--text-main);
	}

	.soul-preview-header-actions {
		display: flex;
		gap: 8px;
	}

	.soul-preview-body {
		margin: 0;
		min-height: 0;
		max-height: min(520px, calc(100vh - 180px));
		overflow: auto;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
		font-size: 12px;
		line-height: 1.6;
		color: var(--text-main);
		background: rgba(15, 23, 42, 0.03);
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 12px;
	}

	.persona-editor-error {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--status-error);
		background: var(--status-error-bg);
		border: 1px solid var(--status-error-border);
		border-radius: 10px;
		padding: 8px 11px;
	}

	.persona-editor-avatar-row {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.persona-editor-avatar-preview {
		width: 64px;
		height: 64px;
		border-radius: 999px;
		object-fit: cover;
		border: 1px solid rgba(15, 23, 42, 0.08);
		flex-shrink: 0;
	}

	.persona-editor-file-input {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}

	.persona-editor-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}
</style>
