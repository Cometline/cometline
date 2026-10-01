import type { ProviderConfig } from '$lib/types';
import type { SettingsSection } from './settings-controller.svelte';

export function filterProviderModels(
	provider: ProviderConfig | undefined,
	search: string
): string[] {
	if (!provider) return [];
	const query = search.trim().toLowerCase();
	if (!query) return provider.models;
	return provider.models.filter((model) => model.toLowerCase().includes(query));
}

export function countEnabledProviders(providers: ProviderConfig[]): number {
	return providers.filter((provider) => provider.enabled).length;
}

export function countEnabledModels(providers: ProviderConfig[]): number {
	return providers.reduce(
		(total, provider) => total + (provider.enabled ? provider.enabledModels.length : 0),
		0
	);
}

export function modelsSectionWarningText(
	activeSection: SettingsSection,
	hasPendingChanges: boolean,
	enabledModelCount: number
): string {
	return activeSection === 'models' && hasPendingChanges && enabledModelCount === 0
		? 'Enable at least one model to send messages.'
		: '';
}
