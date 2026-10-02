<script lang="ts">
	import type { ProviderSettings } from '#lib/types.js';
	import {
		BUILTIN_PERSONAS,
		resolvePersona,
		personaAvatarSrc as builtinPersonaThumbSrc
	} from '#lib/personas/index.js';
	import { personaAvatarCache } from '#lib/personas/avatar-cache.svelte.js';
	import SettingsButton from '../SettingsButton.svelte';
	import type { createSettingsPanelPersonaEditor } from '../../settings-panel-persona-editor.svelte';

	let {
		app,
		editor,
		onSelectPersona
	}: {
		app: ProviderSettings['app'];
		editor: ReturnType<typeof createSettingsPanelPersonaEditor>;
		onSelectPersona: (personaId: string) => void;
	} = $props();

	let resolvedDraftPersona = $derived(resolvePersona(app.personaId, app.personas.custom));
</script>

<section class="settings-panel-frame">
	<div class="settings-section">
		<div class="settings-section-heading">
			<div>
				<h3>Persona</h3>
				<p>Chat avatar, intro animation, and SOUL system prompt</p>
			</div>
			<SettingsButton onclick={editor.openPersonaEditorForCreate}>New persona</SettingsButton>
		</div>
		<div class="icon-variant-options" role="radiogroup" aria-label="Persona">
			{#each BUILTIN_PERSONAS as option (option.id)}
				<button
					type="button"
					class="icon-variant-chip"
					class:selected={app.personaId === option.id}
					role="radio"
					aria-checked={app.personaId === option.id}
					onclick={() => onSelectPersona(option.id)}
				>
					<img
						src={builtinPersonaThumbSrc(
							{
								kind: 'builtin',
								id: option.id,
								label: option.label
							},
							96
						)}
						alt=""
						width="40"
						height="40"
					/>
					<span>{option.label}</span>
				</button>
			{/each}
			{#each editor.customPersonas as persona (persona.id)}
				<div
					class="icon-variant-chip custom-persona-chip"
					class:selected={app.personaId === persona.id}
				>
					<button
						type="button"
						class="custom-persona-select"
						class:selected={app.personaId === persona.id}
						role="radio"
						aria-checked={app.personaId === persona.id}
						onclick={() => onSelectPersona(persona.id)}
					>
						<img
							src={personaAvatarCache.avatarSrcFor(
								{
									kind: 'custom',
									id: persona.id,
									label: persona.name,
									persona
								},
								96
							)}
							alt=""
							width="40"
							height="40"
						/>
						<span>{persona.name}</span>
					</button>
					<div class="custom-persona-actions">
						<button
							type="button"
							class="persona-action-edit"
							onclick={() => editor.openPersonaEditorForEdit(persona)}>Edit</button
						>
						<button
							type="button"
							class="persona-action-delete"
							onclick={() => editor.removeCustomPersona(persona.id)}>Delete</button
						>
					</div>
				</div>
			{/each}
		</div>
		{#if resolvedDraftPersona.kind === 'builtin'}
			<button
				type="button"
				class="soul-preview-trigger"
				onclick={() =>
					editor.openBuiltinSoulPreview(
						resolvedDraftPersona.id,
						resolvedDraftPersona.label
					)}
			>
				View full SOUL for {resolvedDraftPersona.label}
			</button>
		{/if}
	</div>
</section>

<style>
	.icon-variant-options {
		display: flex;
		flex-wrap: wrap;
		gap: 10px;
	}

	.icon-variant-chip {
		display: inline-flex;
		align-items: center;
		gap: 10px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(255, 255, 255, 0.76);
		padding: 8px 12px 8px 8px;
		font: inherit;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.icon-variant-chip img {
		width: 40px;
		height: 40px;
		border-radius: 999px;
		object-fit: cover;
		border: 1px solid rgba(15, 23, 42, 0.08);
	}

	.icon-variant-chip.selected {
		border-color: var(--pane-focus-border);
		box-shadow: 0 0 0 3px var(--pane-focus-glow);
	}

	.icon-variant-chip:hover {
		background: rgba(15, 23, 42, 0.08);
	}

	.custom-persona-chip {
		flex-direction: column;
		align-items: stretch;
		gap: 6px;
		padding: 8px;
	}

	.custom-persona-select {
		display: inline-flex;
		align-items: center;
		gap: 10px;
		border: none;
		background: none;
		padding: 0;
		font: inherit;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
		cursor: pointer;
		border-radius: 10px;
	}

	.custom-persona-select:focus-visible {
		outline: none;
		box-shadow: 0 0 0 3px var(--pane-focus-glow);
	}

	.custom-persona-select img {
		width: 40px;
		height: 40px;
		border-radius: 999px;
		object-fit: cover;
		border: 1px solid rgba(15, 23, 42, 0.08);
	}

	.custom-persona-select.selected {
		color: var(--hero-composer-glow-color);
	}

	.custom-persona-chip.selected {
		border-color: var(--pane-focus-border);
		box-shadow: 0 0 0 3px var(--pane-focus-glow);
	}

	.custom-persona-actions {
		display: flex;
		justify-content: flex-end;
		gap: 6px;
	}

	.custom-persona-actions button {
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.82);
		padding: 4px 9px;
		font: inherit;
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
		cursor: pointer;
		transition:
			background 140ms ease,
			border-color 140ms ease,
			color 140ms ease;
	}

	.custom-persona-actions button:hover {
		background: rgba(15, 23, 42, 0.08);
		color: var(--text-main);
	}

	.custom-persona-actions .persona-action-delete:hover {
		background: rgba(180, 35, 24, 0.08);
		border-color: rgba(180, 35, 24, 0.22);
		color: var(--status-error);
	}

	.soul-preview-trigger {
		align-self: flex-start;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.82);
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
		cursor: pointer;
		transition:
			background 140ms ease,
			border-color 140ms ease;
	}

	.soul-preview-trigger:hover {
		background: rgba(15, 23, 42, 0.08);
		border-color: rgba(15, 23, 42, 0.18);
	}
</style>
