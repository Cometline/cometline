<script lang="ts">
	import { Check, Trash2 } from '@lucide/svelte';
	import SlashCommandMenu from '$lib/components/composer/SlashCommandMenu.svelte';
	import ModelCapabilityIcons from '$lib/components/model/ModelCapabilityIcons.svelte';
	import { formatContextWindow } from '$lib/context-window';
	import { jobMenuSubtitle } from '$lib/jobs/format-job-label';
	import { modelStore } from '$lib/stores/model.svelte';
	import type { createComposerSlashController } from '$lib/components/composer/composer-slash.svelte';

	type SlashController = ReturnType<typeof createComposerSlashController>;

	let {
		slash,
		menuRef = $bindable<HTMLDivElement | null>(null)
	}: {
		slash: SlashController;
		menuRef?: HTMLDivElement | null;
	} = $props();
</script>

{#if slash.workspaceMenuOpen}
	<SlashCommandMenu ariaLabel="Workspace paths" bind:menuRef>
		{#if slash.workspacePathsLoading && !slash.workspacePathsLoaded}
			<p class="skill-command-empty">Loading workspaces...</p>
		{:else if slash.filteredWorkspaceOptions.length === 0}
			<p class="skill-command-empty">No matching workspaces.</p>
		{:else}
			{#each slash.filteredWorkspaceOptions as option, index (`${option.kind}:${option.label}:${index}`)}
				{#if option.kind === 'workspace'}
					<div
						class="workspace-option-row"
						class:highlighted={index === slash.workspaceHighlight}
						data-workspace-index={index}
						role="presentation"
						onpointerenter={() => {
							slash.workspaceHighlight = index;
						}}
					>
						<button
							type="button"
							class="skill-command-option"
							class:highlighted={index === slash.workspaceHighlight}
							role="option"
							aria-selected={index === slash.workspaceHighlight}
							onclick={() => {
								void slash.selectWorkspaceOption(option);
							}}
						>
							<span class="skill-command-name">{option.label}</span>
							<span class="skill-command-description">{option.description}</span>
						</button>
						{#if option.deletable}
							<button
								type="button"
								class="workspace-delete-btn"
								aria-label={`Remove ${option.label} from workspace list`}
								disabled={slash.workspaceDeleting}
								onclick={(event) => {
									void slash.removeWorkspaceFromList(option.path, event);
								}}
							>
								<Trash2 size={13} stroke-width={2} />
							</button>
						{/if}
					</div>
				{:else}
					<button
						type="button"
						class="skill-command-option"
						class:highlighted={index === slash.workspaceHighlight}
						data-workspace-index={index}
						role="option"
						aria-selected={index === slash.workspaceHighlight}
						onpointerenter={() => {
							slash.workspaceHighlight = index;
						}}
						onclick={() => {
							void slash.selectWorkspaceOption(option);
						}}
					>
						<span class="skill-command-name">{option.label}</span>
						<span class="skill-command-description">{option.description}</span>
					</button>
				{/if}
			{/each}
		{/if}
	</SlashCommandMenu>
{:else if slash.modelCommandMenuOpen}
	<SlashCommandMenu ariaLabel="Select model" class="model-command-menu" bind:menuRef>
		{#if slash.filteredModelCommandOptions.length === 0}
			<p class="skill-command-empty">No matching models.</p>
		{:else}
			{#each slash.groupedModelCommandOptions as group (group.providerId)}
				<div class="model-command-group">
					<p class="slash-group-heading">{group.providerName}</p>
					{#each group.options as option (option.id)}
						{@const flatIndex = slash.filteredModelCommandOptions.indexOf(option)}
						<button
							type="button"
							class="skill-command-option model-command-option"
							class:highlighted={flatIndex === slash.modelCommandHighlight}
							class:is-selected={option.id === modelStore.selected?.id}
							data-model-index={flatIndex}
							role="option"
							aria-selected={flatIndex === slash.modelCommandHighlight}
							onpointerenter={() => {
								slash.modelCommandHighlight = flatIndex;
							}}
							onclick={() => {
								void slash.selectModelCommandOption(option);
							}}
						>
							<span class="skill-command-name model-command-name">
								<span class="model-command-label">
									{option.label}
									<ModelCapabilityIcons
										modalities={option.inputModalities}
										known={option.visionKnown}
									/>
								</span>
								{#if option.context && option.context > 0}
									<span class="model-command-context-hint">
										· {formatContextWindow(option.context)} ctx
									</span>
								{/if}
							</span>
							<span class="skill-command-description">{option.modelId}</span>
							{#if option.id === modelStore.selected?.id}
								<span class="model-command-check"
									><Check size={14} stroke-width={2} /></span
								>
							{/if}
						</button>
					{/each}
				</div>
			{/each}
		{/if}
	</SlashCommandMenu>
{:else if slash.jobCommandMenuOpen}
	<SlashCommandMenu ariaLabel="Select job" class="job-command-menu" bind:menuRef>
		{#if slash.jobsLoading && !slash.jobsLoaded}
			<p class="skill-command-empty">Loading jobs…</p>
		{:else if slash.filteredJobOptions.length === 0}
			<p class="skill-command-empty">No ready jobs.</p>
		{:else}
			{#each slash.filteredJobOptions as job, index (job.id)}
				<button
					type="button"
					class="skill-command-option"
					class:highlighted={index === slash.jobCommandHighlight}
					data-job-index={index}
					role="option"
					aria-selected={index === slash.jobCommandHighlight}
					onpointerenter={() => {
						slash.jobCommandHighlight = index;
					}}
					onclick={() => {
						void slash.selectJobCommandOption(job);
					}}
				>
					<span class="skill-command-name">{job.description}</span>
					{#if jobMenuSubtitle(job)}
						<span class="skill-command-description">{jobMenuSubtitle(job)}</span>
					{/if}
				</button>
			{/each}
		{/if}
	</SlashCommandMenu>
{:else if slash.skillMenuOpen}
	<SlashCommandMenu ariaLabel="Skill commands" bind:menuRef>
		{#if slash.skillsLoading && !slash.skillsLoaded}
			<p class="skill-command-empty">Loading skills...</p>
		{:else if slash.filteredSlashOptions.length === 0}
			<p class="skill-command-empty">No matching skills.</p>
		{:else}
			{#each slash.filteredSlashOptions as option, index (option.kind + ':' + option.name)}
				{#if index === 0 || slash.filteredSlashOptions[index - 1].kind !== option.kind}
					<p class="slash-group-heading">
						{option.kind === 'builtin' ? 'System commands' : 'Skills'}
					</p>
				{/if}
				<button
					type="button"
					class="skill-command-option"
					class:highlighted={index === slash.skillHighlight}
					data-skill-index={index}
					role="option"
					aria-selected={index === slash.skillHighlight}
					onpointerenter={() => (slash.skillHighlight = index)}
					onpointerdown={(e) => {
						e.preventDefault();
						slash.selectSlashOption(option);
					}}
				>
					<span class="skill-command-name">/{option.name}</span>
					<span class="skill-command-description">{option.description}</span>
				</button>
			{/each}
		{/if}
	</SlashCommandMenu>
{/if}
