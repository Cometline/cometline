<script lang="ts">
	import type { ProviderConfig } from '$lib/types';
	import { providerOptionLabel } from '$lib/features/settings/model-roles-panel-roles';

	let {
		title,
		description,
		providerLabel,
		modelLabel,
		hint,
		providers,
		providerId,
		modelId,
		models,
		onProviderChange,
		onModelChange
	}: {
		title: string;
		description: string;
		providerLabel: string;
		modelLabel: string;
		hint: string;
		providers: ProviderConfig[];
		providerId: string;
		modelId: string;
		models: string[];
		onProviderChange: (providerId: string) => void;
		onModelChange: (modelId: string) => void;
	} = $props();
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>{title}</h3>
		<p>{description}</p>
	</div>
	<label>
		<span>{providerLabel}</span>
		<select value={providerId} onchange={(e) => onProviderChange(e.currentTarget.value)}>
			<option value="">Use default model</option>
			{#each providers as provider (provider.id)}
				<option value={provider.id}>{providerOptionLabel(provider)}</option>
			{/each}
		</select>
	</label>
	{#if providerId}
		<label>
			<span>{modelLabel}</span>
			<select
				value={modelId || models[0] || ''}
				onchange={(e) => onModelChange(e.currentTarget.value)}
			>
				{#each models as model (model)}
					<option value={model}>{model}</option>
				{/each}
			</select>
			<p class="settings-field-hint">{hint}</p>
		</label>
	{/if}
</div>
