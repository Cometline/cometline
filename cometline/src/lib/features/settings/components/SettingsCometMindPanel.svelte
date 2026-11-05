<script lang="ts">
	import SettingsCometMindQueue from './cometmind/SettingsCometMindQueue.svelte';
	import SettingsCometMindRuntime from './cometmind/SettingsCometMindRuntime.svelte';
	import SettingsCometMindHooks from './cometmind/SettingsCometMindHooks.svelte';
	import SettingsCometMindSkills from './cometmind/SettingsCometMindSkills.svelte';
	import { formatIdList, parseIdList, type CometMindSettings } from '#lib/cometmind-settings.js';
	import type { ProviderConfig } from '#lib/types.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';
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
	let gatewayRunning = $state(false);
	let gatewayBusy = $state(false);

	onMount(() => {
		void refreshGatewayStatus();
	});

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
		<SettingsCometMindHooks bind:cometmind />
		<SettingsCometMindSkills bind:cometmind />
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
