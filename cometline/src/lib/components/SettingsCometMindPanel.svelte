<script lang="ts">
	import { FolderOpen } from '@lucide/svelte';
	import SettingsToggle from '$lib/components/SettingsToggle.svelte';
	import { formatIdList, parseIdList, type CometMindSettings } from '$lib/cometmind-settings';
	import type { ProviderConfig } from '$lib/types';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { listSkills, syncSkills, deleteSkill, exportSkill } from '$lib/client/cometmind';
	import type { SkillResource } from '$lib/types';
	import { onMount } from 'svelte';

	let {
		cometmind = $bindable(),
		providers = [],
		onPickWorkspace
	}: {
		cometmind: CometMindSettings;
		providers?: ProviderConfig[];
		onPickWorkspace?: () => void | Promise<void>;
	} = $props();

	const runtimeProviders = $derived(
		providers.filter((provider) => provider.enabled && provider.enabledModels.length > 0)
	);

	const discordProvider = $derived(
		runtimeProviders.find((provider) => provider.id === cometmind.gateway.discord.providerId) ??
			runtimeProviders[0] ??
			providers.find((provider) => provider.id === cometmind.gateway.discord.providerId) ??
			providers[0]
	);

	const discordModels = $derived(
		discordProvider?.enabledModels.length
			? discordProvider.enabledModels
			: (discordProvider?.models ?? [])
	);

	function setDiscordProvider(providerId: string) {
		const provider = providers.find((item) => item.id === providerId);
		if (!provider) return;
		const modelId =
			provider.enabledModels[0] ?? provider.selectedModel ?? provider.models[0] ?? '';
		cometmind = {
			...cometmind,
			gateway: {
				discord: {
					...cometmind.gateway.discord,
					providerId,
					modelId
				}
			}
		};
	}

	function setDiscordModel(modelId: string) {
		cometmind = {
			...cometmind,
			gateway: {
				discord: {
					...cometmind.gateway.discord,
					modelId
				}
			}
		};
	}

	let allowedUsersText = $state(formatIdList(cometmind.gateway.discord.allowedUsers));
	let allowedChannelsText = $state(formatIdList(cometmind.gateway.discord.allowedChannels));
	let argsText = $state(cometmind.acp.args.join(' '));
	let rootsText = $state(formatIdList(cometmind.skills.roots));
	let skills = $state<SkillResource[]>([]);
	let skillErrors = $state<string[]>([]);
	let skillsBusy = $state(false);
	let skillsStatus = $state('');
	let deletePending = $state<string | null>(null);
	let gatewayRunning = $state(false);
	let gatewayBusy = $state(false);

	onMount(() => {
		void refreshGatewayStatus();
		void refreshSkills();
	});

	async function refreshSkills() {
		skillsBusy = true;
		skillsStatus = '';
		try {
			const result = await listSkills(shellStore.workspacePath);
			skills = result.skills;
			skillErrors = result.errors ?? [];
		} catch (err) {
			skillsStatus = err instanceof Error ? err.message : 'Failed to load skills';
		} finally {
			skillsBusy = false;
		}
	}

	async function onSyncSkills() {
		skillsBusy = true;
		skillsStatus = '';
		try {
			const result = await syncSkills(shellStore.workspacePath);
			skillsStatus = `Synced ${result.created.length} skills, skipped ${result.skipped.length}.`;
			await refreshSkills();
		} catch (err) {
			skillsStatus = err instanceof Error ? err.message : 'Failed to sync skills';
		} finally {
			skillsBusy = false;
		}
	}

	async function onExportSkill(name: string) {
		skillsBusy = true;
		skillsStatus = '';
		try {
			const blob = await exportSkill(name, shellStore.workspacePath);
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = `${name}.zip`;
			link.click();
			URL.revokeObjectURL(url);
			skillsStatus = `Exported ${name}.zip`;
		} catch (err) {
			skillsStatus = err instanceof Error ? err.message : 'Failed to export skill';
		} finally {
			skillsBusy = false;
		}
	}

	function requestDeleteSkill(name: string) {
		deletePending = name;
	}

	function cancelDeleteSkill() {
		deletePending = null;
	}

	async function confirmDeleteSkill(name: string) {
		skillsBusy = true;
		skillsStatus = '';
		try {
			await deleteSkill(name, shellStore.workspacePath);
			deletePending = null;
			skillsStatus = `Deleted skill ${name}.`;
			await refreshSkills();
		} catch (err) {
			skillsStatus = err instanceof Error ? err.message : 'Failed to delete skill';
		} finally {
			skillsBusy = false;
		}
	}

	async function refreshGatewayStatus() {
		const status = await window.electronAPI?.getDiscordGatewayStatus?.();
		if (!status) return;
		gatewayRunning = status.running;
		cometmind = {
			...cometmind,
			gateway: {
				discord: {
					...cometmind.gateway.discord,
					enabled: status.enabled
				}
			}
		};
	}

	async function onDiscordGatewayToggle(enabled: boolean) {
		if (!window.electronAPI?.setDiscordGatewayEnabled) return;
		gatewayBusy = true;
		try {
			const result = await window.electronAPI.setDiscordGatewayEnabled(enabled);
			gatewayRunning = result.running;
			cometmind = {
				...cometmind,
				gateway: {
					discord: {
						...cometmind.gateway.discord,
						enabled: result.enabled
					}
				}
			};
		} finally {
			gatewayBusy = false;
		}
	}

	export function syncFields() {
		syncListsFromText();
	}

	function syncListsFromText() {
		cometmind = {
			...cometmind,
			acp: {
				...cometmind.acp,
				args: argsText
					.split(/\s+/)
					.map((part) => part.trim())
					.filter(Boolean)
			},
			skills: {
				...cometmind.skills,
				roots: parseIdList(rootsText)
			},
			gateway: {
				discord: {
					...cometmind.gateway.discord,
					allowedUsers: parseIdList(allowedUsersText),
					allowedChannels: parseIdList(allowedChannelsText)
				}
			}
		};
	}

	function useCurrentWorkspace() {
		if (!shellStore.workspacePath) return;
		cometmind = {
			...cometmind,
			gateway: {
				discord: {
					...cometmind.gateway.discord,
					workspacePath: shellStore.workspacePath
				}
			}
		};
	}
</script>

<section class="cometmind-panel">
	<div class="section-block">
		<div class="section-heading">
			<h3>Runtime</h3>
			<p>
				Controls sent to CometMind for each agent response. Settings are saved to
				<code>~/.cometmind/cometline-settings.json</code>.
			</p>
		</div>
		<label>
			<span>Max output tokens</span>
			<input
				type="number"
				bind:value={cometmind.maxTokens}
				min="1"
				step="1"
				placeholder="2048"
			/>
			<p class="field-hint">
				Caps the model's generated response length. Lower values reduce worst-case latency
				and cost.
			</p>
		</label>
	</div>

	<div class="section-block">
		<div class="section-heading">
			<h3>OpenCode subagent (ACP)</h3>
			<p>
				Delegate coding tasks to the local OpenCode CLI. Written to <code>[acp]</code> in
				<code>~/.cometmind/cometline-settings.json</code>.
			</p>
		</div>
		<label>
			<span>Command path</span>
			<input
				type="text"
				bind:value={cometmind.acp.command}
				placeholder="opencode"
				spellcheck="false"
			/>
		</label>
		<label>
			<span>Arguments (space-separated)</span>
			<input
				type="text"
				bind:value={argsText}
				onchange={syncListsFromText}
				onblur={syncListsFromText}
				placeholder="acp"
				spellcheck="false"
			/>
		</label>
		<label>
			<span>Timeout</span>
			<input
				type="text"
				bind:value={cometmind.acp.timeout}
				placeholder="30m"
				spellcheck="false"
			/>
		</label>
	</div>

	<div class="section-block">
		<div class="section-heading">
			<h3>Skills</h3>
			<p>
				CometMind reads Agent Skills from <code>~/.cometmind/skills</code>, workspace
				<code>.agents/skills</code>/<code>.claude/skills</code>, and optional OpenCode or
				Claude global skill folders.
			</p>
		</div>
		<SettingsToggle
			label="Enable skills"
			description="Expose a compact skill index to CometMind and allow read-only loading via load_skill."
			bind:checked={cometmind.skills.enabled}
		/>
		<div class="skills-options">
			<label class="checkbox-row">
				<input type="checkbox" bind:checked={cometmind.skills.includeOpenCode} />
				<span>Include OpenCode skills</span>
			</label>
			<label class="checkbox-row">
				<input type="checkbox" bind:checked={cometmind.skills.includeClaude} />
				<span>Include Claude Code skills</span>
			</label>
			<label class="checkbox-row">
				<input type="checkbox" bind:checked={cometmind.skills.mirrorToCometMind} />
				<span>Mirror symlinks to ~/.cometmind/skills when syncing</span>
			</label>
		</div>
		<label>
			<span>Additional skill roots (one per line)</span>
			<textarea
				bind:value={rootsText}
				onchange={syncListsFromText}
				onblur={syncListsFromText}
				rows="3"
				placeholder="/path/to/skills"
				spellcheck="false"
			></textarea>
		</label>
		<div class="skills-actions">
			<button class="secondary" type="button" onclick={refreshSkills} disabled={skillsBusy}>
				{skillsBusy ? 'Loading...' : 'Refresh skills'}
			</button>
			<button class="secondary" type="button" onclick={onSyncSkills} disabled={skillsBusy}>
				Sync symlinks
			</button>
		</div>
		{#if skillsStatus}
			<p class="field-hint">{skillsStatus}</p>
		{/if}
		<div class="skills-list">
			<div class="skills-list-header">
				<span>Available skills</span>
				<strong>{skills.length}</strong>
			</div>
			{#if skills.length === 0}
				<p class="field-hint">
					No skills discovered yet. Try <code>npx skills add ...</code> or add a custom root.
				</p>
			{:else}
				{#each skills as skill (skill.name)}
					<div class="skill-row" title={skill.path}>
						<div class="skill-row-main">
							<strong>{skill.name}</strong>
							{#if skill.is_symlink}
								<span class="skill-badge">symlink</span>
							{/if}
							<p>{skill.description}</p>
						</div>
						<div class="skill-row-actions">
							{#if deletePending === skill.name}
								<span class="skill-delete-prompt">Delete {skill.name}?</span>
								<button
									class="secondary danger"
									type="button"
									disabled={skillsBusy}
									onclick={() => confirmDeleteSkill(skill.name)}
								>
									Confirm
								</button>
								<button
									class="secondary"
									type="button"
									disabled={skillsBusy}
									onclick={cancelDeleteSkill}
								>
									Cancel
								</button>
							{:else}
								{#if skill.can_export}
									<button
										class="secondary"
										type="button"
										disabled={skillsBusy}
										onclick={() => onExportSkill(skill.name)}
									>
										Export
									</button>
								{/if}
								<button
									class="secondary danger"
									type="button"
									disabled={skillsBusy || !skill.can_delete}
									title={skill.can_delete
										? 'Delete from ~/.cometmind/skills'
										: 'External or symlink skills cannot be deleted'}
									onclick={() => requestDeleteSkill(skill.name)}
								>
									Delete
								</button>
							{/if}
						</div>
					</div>
				{/each}
			{/if}
		</div>
		{#if skillErrors.length > 0}
			<div class="skill-errors">
				{#each skillErrors as error}
					<p>{error}</p>
				{/each}
			</div>
		{/if}
	</div>

	<div class="section-block">
		<div class="section-heading">
			<h3>Discord gateway</h3>
			<p>
				Runs <code>cometmind gateway run --platform discord</code> while Cometline is open.
				Settings are saved to <code>~/.cometmind/cometline-settings.json</code>.
			</p>
		</div>
		<div class="gateway-runtime">
			<SettingsToggle
				label="Run Discord gateway"
				description="Start the Discord bot automatically while this app is running."
				bind:checked={cometmind.gateway.discord.enabled}
				disabled={gatewayBusy || !window.electronAPI?.setDiscordGatewayEnabled}
				onchange={onDiscordGatewayToggle}
			/>
			<p class="gateway-status" class:running={gatewayRunning}>
				Status: {gatewayRunning ? 'Running' : 'Stopped'}
			</p>
		</div>
		<label>
			<span>Bot Token</span>
			<input
				type="password"
				bind:value={cometmind.gateway.discord.botToken}
				placeholder="Paste from Discord Developer Portal"
				spellcheck="false"
				autocomplete="off"
			/>
		</label>
		<label>
			<span>Default provider</span>
			<select
				value={cometmind.gateway.discord.providerId || discordProvider?.id || ''}
				onchange={(e) => setDiscordProvider(e.currentTarget.value)}
			>
				{#each providers as provider (provider.id)}
					<option value={provider.id}>{provider.name}</option>
				{/each}
			</select>
		</label>
		<label>
			<span>Default model</span>
			<select
				value={cometmind.gateway.discord.modelId || discordModels[0] || ''}
				onchange={(e) => setDiscordModel(e.currentTarget.value)}
			>
				{#each discordModels as model (model)}
					<option value={model}>{model}</option>
				{/each}
			</select>
			<p class="field-hint">
				Used for new Discord / thread sessions. Falls back to the global CometMind model
				when empty.
			</p>
		</label>
		<label>
			<span>Workspace path (repo for the gateway)</span>
			<div class="path-row">
				<input
					type="text"
					bind:value={cometmind.gateway.discord.workspacePath}
					placeholder="/path/to/cometline-release"
					spellcheck="false"
				/>
				<button class="secondary" type="button" onclick={useCurrentWorkspace}
					>Current workspace</button
				>
				{#if onPickWorkspace}
					<button
						class="secondary icon"
						type="button"
						aria-label="Choose folder"
						onclick={onPickWorkspace}
					>
						<FolderOpen size={14} />
					</button>
				{/if}
			</div>
		</label>
		<label>
			<span>Allowed user IDs (one per line)</span>
			<textarea
				bind:value={allowedUsersText}
				onchange={syncListsFromText}
				onblur={syncListsFromText}
				rows="3"
				placeholder="123456789012345678"
				spellcheck="false"
			></textarea>
		</label>
		<label>
			<span>Allowed channel IDs (one per line; leave empty for no channel restriction)</span>
			<textarea
				bind:value={allowedChannelsText}
				onchange={syncListsFromText}
				onblur={syncListsFromText}
				rows="3"
				placeholder="987654321098765432"
				spellcheck="false"
			></textarea>
		</label>
		<label class="checkbox-row">
			<input type="checkbox" bind:checked={cometmind.gateway.discord.requireMention} />
			<span>Require @mention in server channels</span>
		</label>
	</div>
</section>

<style>
	.cometmind-panel {
		display: flex;
		flex-direction: column;
		gap: 28px;
	}

	.section-block {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.section-heading h3 {
		margin: 0 0 4px;
		font-size: 15px;
		font-weight: 650;
		color: var(--text-main);
	}

	.section-heading p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.section-heading code {
		font-size: 11px;
	}

	.gateway-runtime {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.gateway-status {
		margin: 0;
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.gateway-status.running {
		color: #2f6f4f;
	}

	.skills-options,
	.skills-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 10px 14px;
	}

	.skills-list {
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.58);
		max-height: 260px;
		overflow: auto;
	}

	.skills-list-header {
		position: sticky;
		top: 0;
		display: flex;
		justify-content: space-between;
		padding: 9px 11px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(250, 248, 244, 0.94);
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
	}

	.skill-row {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		padding: 10px 11px;
		border-bottom: 1px solid rgba(0, 0, 0, 0.06);
	}

	.skill-row-main {
		min-width: 0;
		flex: 1;
	}

	.skill-row-actions {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.skill-badge {
		display: inline-block;
		margin-left: 6px;
		padding: 1px 6px;
		border-radius: 999px;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-muted);
		background: rgba(15, 23, 42, 0.06);
		vertical-align: middle;
	}

	.skill-delete-prompt {
		font-size: 12px;
		color: var(--text-muted);
	}

	.skill-row .secondary.danger {
		color: #b42318;
	}

	.skill-row:last-child {
		border-bottom: 0;
	}

	.skill-row strong {
		display: block;
		font-size: 12px;
		color: var(--text-main);
	}

	.skill-row p,
	.skill-errors p {
		margin: 3px 0 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.skill-errors {
		border: 1px solid rgba(190, 90, 60, 0.25);
		border-radius: 10px;
		padding: 8px 10px;
		background: rgba(255, 236, 224, 0.45);
	}

	label {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 12px;
		color: var(--text-muted);
	}

	label span {
		font-weight: 600;
		color: var(--text-main);
	}

	.field-hint {
		margin: 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	input[type='text'],
	input[type='password'],
	select,
	textarea {
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 9px 11px;
		font-size: 13px;
		color: var(--text-main);
		background: rgba(255, 255, 255, 0.82);
	}

	textarea {
		resize: vertical;
		min-height: 72px;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 12px;
	}

	.checkbox-row {
		flex-direction: row;
		align-items: center;
		gap: 8px;
	}

	.path-row {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.path-row input {
		flex: 1;
		min-width: 0;
	}

	.secondary {
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.82);
		color: var(--text-main);
		border-radius: 9px;
		padding: 8px 10px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		white-space: nowrap;
	}

	.secondary.icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 8px;
	}

	.secondary:hover {
		background: white;
	}
</style>
