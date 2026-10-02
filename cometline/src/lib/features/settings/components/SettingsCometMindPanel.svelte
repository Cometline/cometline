<script lang="ts">
	import SettingsCometMindQueue from './cometmind/SettingsCometMindQueue.svelte';
	import SettingsCometMindRuntime from './cometmind/SettingsCometMindRuntime.svelte';
	import SettingsCometMindSkills from './cometmind/SettingsCometMindSkills.svelte';
	import { formatIdList, parseIdList, type CometMindSettings } from '#lib/cometmind-settings.js';
	import type { ProviderConfig } from '#lib/types.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';

	import { listSkills, syncSkills, deleteSkill, exportSkill } from '#lib/client/cometmind.js';
	import type { SkillResource } from '#lib/types.js';
	import { onMount } from 'svelte';
	import ConfirmActionModal from '#lib/components/ConfirmActionModal.svelte';
	import SettingsMCPPanel from './SettingsMCPPanel.svelte';

	let {
		cometmind = $bindable(),
		providers = [],
		onPickWorkspace,
		onPersistBeforeRuntimeAction
	}: {
		cometmind: CometMindSettings;
		providers?: ProviderConfig[];
		onPickWorkspace?: () => void | Promise<void>;
		onPersistBeforeRuntimeAction?: (
			overrides?: Partial<Pick<CometMindSettings, 'mcp'>>
		) => Promise<void>;
	} = $props();

	type SkillSourceFilter =
		| 'all'
		| 'cometmind'
		| 'global'
		| 'workspace'
		| 'opencode'
		| 'claude'
		| 'other';

	const SKILL_SOURCE_FILTERS: { id: SkillSourceFilter; label: string }[] = [
		{ id: 'all', label: 'All' },
		{ id: 'cometmind', label: 'CometMind' },
		{ id: 'global', label: 'Global Agent Skills' },
		{ id: 'workspace', label: 'Workspace' },
		{ id: 'opencode', label: 'OpenCode' },
		{ id: 'claude', label: 'Claude Code' },
		{ id: 'other', label: 'Other' }
	];

	const SKILL_SOURCE_LABELS: Record<Exclude<SkillSourceFilter, 'all'>, string> = {
		cometmind: 'CometMind',
		global: 'Global Agent Skills',
		workspace: 'Workspace',
		opencode: 'OpenCode',
		claude: 'Claude Code',
		other: 'Other'
	};

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
	let skills = $state<SkillResource[]>([]);
	let skillErrors = $state<string[]>([]);
	let skillsBusy = $state(false);
	let skillsStatus = $state('');
	let deletePending = $state<SkillResource | null>(null);
	let gatewayRunning = $state(false);
	let gatewayBusy = $state(false);
	let mcpPanel: SettingsMCPPanel | undefined = $state();
	let skillSearch = $state('');
	let skillSourceFilter = $state<SkillSourceFilter>('all');

	function normalizeSkillPath(path: string) {
		return path.replace(/\\/g, '/').toLowerCase();
	}

	function skillSourceCategory(skill: SkillResource): Exclude<SkillSourceFilter, 'all'> {
		const path = normalizeSkillPath(skill.path);
		if (path.includes('/.cometmind/skills/')) return 'cometmind';
		if (path.includes('/.config/opencode/skills/')) return 'opencode';
		if (path.includes('/.agents/skills/')) {
			const workspacePath = shellStore.workspacePath
				? normalizeSkillPath(shellStore.workspacePath)
				: '';
			return workspacePath && path.startsWith(`${workspacePath}/`) ? 'workspace' : 'global';
		}
		if (path.includes('/.claude/skills/')) {
			const workspacePath = shellStore.workspacePath
				? normalizeSkillPath(shellStore.workspacePath)
				: '';
			if (workspacePath && path.startsWith(`${workspacePath}/`)) return 'workspace';
			return 'claude';
		}
		return 'other';
	}

	let filteredSkills = $derived.by(() => {
		const query = skillSearch.trim().toLowerCase();
		return skills.filter((skill) => {
			if (skillSourceFilter !== 'all' && skillSourceCategory(skill) !== skillSourceFilter) {
				return false;
			}
			if (!query) return true;
			const sourceLabel = SKILL_SOURCE_LABELS[skillSourceCategory(skill)].toLowerCase();
			return (
				skill.name.toLowerCase().includes(query) ||
				skill.description.toLowerCase().includes(query) ||
				skill.path.toLowerCase().includes(query) ||
				sourceLabel.includes(query)
			);
		});
	});

	let skillSourceCounts = $derived.by(() => {
		const counts: Record<SkillSourceFilter, number> = {
			all: skills.length,
			cometmind: 0,
			global: 0,
			workspace: 0,
			opencode: 0,
			claude: 0,
			other: 0
		};
		for (const skill of skills) {
			counts[skillSourceCategory(skill)] += 1;
		}
		return counts;
	});

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

	function requestDeleteSkill(skill: SkillResource) {
		deletePending = skill;
	}

	async function confirmDeleteSkill() {
		const skill = deletePending;
		if (!skill) return;
		skillsBusy = true;
		skillsStatus = '';
		try {
			await deleteSkill(skill.name, shellStore.workspacePath);
			deletePending = null;
			skillsStatus = `Deleted skill ${skill.name}.`;
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
		mcpPanel?.syncFields();
	}

	// Flushes the local text mirrors (allowedUsersText /
	// allowedChannelsText) into the bound `cometmind` draft. The inputs wire this
	// to `oninput` (not just change/blur) so editing one of these fields enables
	// the Save button immediately — without it the first Save click can land
	// while the button is still disabled. `syncFields()` keeps it as a save-time
	// backstop.
	function syncListsFromText() {
		cometmind = {
			...cometmind,
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

<section class="cometmind-panel settings-panel-frame">
	<div class="settings-panel-body">
		<SettingsCometMindRuntime bind:cometmind />
		<SettingsCometMindSkills
			bind:cometmind
			bind:skillSearch
			bind:skillSourceFilter
			{skills}
			{filteredSkills}
			{skillErrors}
			{skillsBusy}
			{skillsStatus}
			{skillSourceCounts}
			filters={SKILL_SOURCE_FILTERS}
			sourceLabels={SKILL_SOURCE_LABELS}
			{skillSourceCategory}
			{refreshSkills}
			{onSyncSkills}
			{onExportSkill}
			{requestDeleteSkill}
		/>
		<SettingsMCPPanel
			bind:this={mcpPanel}
			bind:mcp={cometmind.mcp}
			{onPersistBeforeRuntimeAction}
		/>
		<SettingsCometMindQueue
			bind:cometmind
			bind:allowedUsersText
			bind:allowedChannelsText
			{providers}
			{discordProvider}
			{discordModels}
			{gatewayRunning}
			{gatewayBusy}
			{onPickWorkspace}
			{setDiscordProvider}
			{setDiscordModel}
			{onDiscordGatewayToggle}
			{syncListsFromText}
			{useCurrentWorkspace}
		/>
	</div>
</section>

<ConfirmActionModal
	open={Boolean(deletePending)}
	title={`Delete "${deletePending?.name ?? ''}"?`}
	description={deletePending
		? `This removes the original files at ${deletePending.path}. This cannot be undone.`
		: ''}
	confirmLabel="Delete"
	onConfirm={() => void confirmDeleteSkill()}
	onCancel={() => (deletePending = null)}
/>
