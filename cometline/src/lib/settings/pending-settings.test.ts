import { describe, expect, it } from 'vitest';
import { defaultSettings, normalizeSettings } from '$lib/settings/schema';
import {
	pendingSettingsSnapshot,
	sectionPendingDirty,
	settingsPendingDirty
} from './pending-settings';

describe('pending-settings', () => {
	it('ignores instant-tier fields when comparing pending dirty state', () => {
		const base = normalizeSettings(defaultSettings());
		const draft = normalizeSettings({
			...base,
			shortcuts: {
				...base.shortcuts,
				openSettings: { key: 'k', meta: true, ctrl: false, shift: false, alt: false }
			},
			app: { ...base.app, openAtLogin: !base.app.openAtLogin },
			cometmind: {
				...base.cometmind,
				gateway: {
					discord: {
						...base.cometmind.gateway.discord,
						enabled: !base.cometmind.gateway.discord.enabled
					}
				}
			}
		});

		expect(settingsPendingDirty(draft, base)).toBe(false);
	});

	it('detects pending provider edits', () => {
		const base = normalizeSettings(defaultSettings());
		const providerId = 'openai-compatible';
		const draft = normalizeSettings({
			...base,
			providers: base.providers.map((provider) =>
				provider.id === providerId ? { ...provider, name: 'Renamed provider' } : provider
			)
		});

		expect(settingsPendingDirty(draft, base)).toBe(true);
		expect(sectionPendingDirty('models', draft, base)).toBe(true);
		expect(sectionPendingDirty('shortcuts', draft, base)).toBe(false);
	});

	it('detects pending terminal appearance edits in the Appearance section', () => {
		const base = normalizeSettings(defaultSettings());
		const draft = normalizeSettings({
			...base,
			appearance: {
				...base.appearance,
				terminal: { ...base.appearance.terminal, fontSize: 14 }
			}
		});

		expect(settingsPendingDirty(draft, base)).toBe(true);
		expect(sectionPendingDirty('appearance', draft, base)).toBe(true);
	});

	it('detects pending response complete sound edits in the Appearance section', () => {
		const base = normalizeSettings(defaultSettings());
		const draft = normalizeSettings({
			...base,
			appearance: {
				...base.appearance,
				responseCompleteSound: {
					...base.appearance.responseCompleteSound,
					volume: 0.25
				}
			}
		});

		expect(settingsPendingDirty(draft, base)).toBe(true);
		expect(sectionPendingDirty('appearance', draft, base)).toBe(true);
	});

	it('produces stable snapshots', () => {
		const settings = normalizeSettings(defaultSettings());
		expect(pendingSettingsSnapshot(settings)).toBe(pendingSettingsSnapshot(settings));
	});
});
