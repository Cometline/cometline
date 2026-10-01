<script lang="ts">
	import type { CometMindSettings } from '$lib/cometmind-settings';
	import type { ProviderConfig } from '$lib/types';
	import {
		modelsForProvider,
		providerById,
		withAutonomyModel,
		withAutonomyProvider,
		withExtractionModel,
		withExtractionProvider,
		withSynthesisModel,
		withSynthesisProvider,
		withTitleModel,
		withTitleProvider
	} from '$lib/features/settings/model-roles-panel-roles';
	import DefaultModelPicker from './model-roles/DefaultModelPicker.svelte';
	import ModelRoleSection from './model-roles/ModelRoleSection.svelte';

	let {
		cometmind = $bindable(),
		defaultModelId = $bindable(''),
		defaultProviderId = $bindable(''),
		providers = []
	}: {
		cometmind: CometMindSettings;
		defaultModelId: string;
		defaultProviderId: string;
		providers?: ProviderConfig[];
	} = $props();

	const runtimeProviders = $derived(
		providers.filter((provider) => provider.enabled && provider.enabledModels.length > 0)
	);

	const titleModels = $derived(
		modelsForProvider(providerById(runtimeProviders, cometmind.titleProviderId))
	);
	const extractionModels = $derived(
		modelsForProvider(providerById(runtimeProviders, cometmind.memory.extractionProviderId))
	);
	const autonomyModels = $derived(
		modelsForProvider(providerById(runtimeProviders, cometmind.autonomy.providerId))
	);
	const synthesisModels = $derived(
		modelsForProvider(providerById(runtimeProviders, cometmind.skills.synthesisProviderId))
	);
</script>

<section class="model-roles-panel settings-panel-frame">
	<div class="settings-panel-body">
		<div class="settings-section">
			<div class="settings-section-heading">
				<div>
					<h3>Default model</h3>
					<p>
						Choose which model new chats use by default, and what unpinned roles
						(titles, extraction, jobs, synthesis) fall back to. You can still switch
						models per session.
					</p>
				</div>
			</div>
			<DefaultModelPicker bind:defaultModelId bind:defaultProviderId {providers} />
		</div>

		<ModelRoleSection
			title="Autonomous jobs"
			description="Optional override for background job claims. Leave empty to use the Default model."
			providerLabel="Autonomous jobs provider"
			modelLabel="Autonomous jobs model"
			hint="Pick a reliable coding-capable model. Job runs can execute tools and continue without a visible chat open."
			providers={runtimeProviders}
			providerId={cometmind.autonomy.providerId}
			modelId={cometmind.autonomy.modelId}
			models={autonomyModels}
			onProviderChange={(id) =>
				(cometmind = withAutonomyProvider(cometmind, runtimeProviders, id))}
			onModelChange={(id) => (cometmind = withAutonomyModel(cometmind, id))}
		/>

		<ModelRoleSection
			title="Skill synthesis"
			description="Optional override for skill draft proposals after completed jobs. Leave empty to use the Default model."
			providerLabel="Synthesis provider"
			modelLabel="Synthesis model"
			hint="This model only drafts skills. Drafts still require explicit promotion before becoming active skills."
			providers={runtimeProviders}
			providerId={cometmind.skills.synthesisProviderId}
			modelId={cometmind.skills.synthesisModel}
			models={synthesisModels}
			onProviderChange={(id) =>
				(cometmind = withSynthesisProvider(cometmind, runtimeProviders, id))}
			onModelChange={(id) => (cometmind = withSynthesisModel(cometmind, id))}
		/>

		<ModelRoleSection
			title="Session titles"
			description="CometMind names each session from your first message using an LLM. Pin a cheaper / faster model here, or leave empty to use the Default model."
			providerLabel="Title provider"
			modelLabel="Title model"
			hint="A small, fast model is ideal — titles are short and don't need a frontier model."
			providers={runtimeProviders}
			providerId={cometmind.titleProviderId}
			modelId={cometmind.titleModelId}
			models={titleModels}
			onProviderChange={(id) =>
				(cometmind = withTitleProvider(cometmind, runtimeProviders, id))}
			onModelChange={(id) => (cometmind = withTitleModel(cometmind, id))}
		/>

		<ModelRoleSection
			title="Memory extraction"
			description="After each turn, CometMind extracts durable memories in the background. The same provider also runs memory compaction merges. Leave empty to use the Default model."
			providerLabel="Extraction provider"
			modelLabel="Extraction model"
			hint="A small, fast model is ideal — extraction and compaction both use this pin."
			providers={runtimeProviders}
			providerId={cometmind.memory.extractionProviderId}
			modelId={cometmind.memory.extractionModel}
			models={extractionModels}
			onProviderChange={(id) =>
				(cometmind = withExtractionProvider(cometmind, runtimeProviders, id))}
			onModelChange={(id) => (cometmind = withExtractionModel(cometmind, id))}
		/>
	</div>
</section>
