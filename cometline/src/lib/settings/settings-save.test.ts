import { describe, expect, it } from 'vitest';

import { defaultSettings } from '$lib/settings/schema';

import { runtimeActionForSettingsSave, saveStatusMessage } from './settings-save';

describe('runtimeActionForSettingsSave', () => {
	it('returns reload for provider changes unrelated to memory providers', () => {
		const persisted = defaultSettings();
		const next = defaultSettings();
		next.providers[0] = { ...next.providers[0], enabled: true, apiKey: 'new-key' };

		expect(runtimeActionForSettingsSave(persisted, next)).toBe('reload');
	});

	it('returns restart for memory settings changes', () => {
		const persisted = defaultSettings();
		const next = defaultSettings();
		next.cometmind.memory.extractionModel = 'text-embedding-3-large';

		expect(runtimeActionForSettingsSave(persisted, next)).toBe('restart');
	});

	it('returns restart when a memory provider entry changes', () => {
		const persisted = defaultSettings();
		const next = defaultSettings();
		next.cometmind.memory.embedding.providerId = 'openai';
		next.providers = next.providers.map((provider) =>
			provider.id === 'openai' ? { ...provider, baseURL: 'http://localhost:11434/v1' } : provider
		);

		expect(runtimeActionForSettingsSave(persisted, next)).toBe('restart');
	});
});

describe('saveStatusMessage', () => {
	it('reports reload distinctly from restart when no reload outcome is known', () => {
		expect(saveStatusMessage('agent', 'reload')).toBe('Changes saved. CometMind reloaded.');
		expect(saveStatusMessage('agent', 'restart')).toBe('Changes saved. CometMind restarted.');
		expect(saveStatusMessage('agent', 'none')).toBe('Changes saved.');
	});

	it('reports a confirmed reload the same as before when reload succeeded', () => {
		const message = saveStatusMessage('agent', 'reload', false, {
			action: 'reload',
			healthy: true
		});
		expect(message).toBe('Changes saved. CometMind reloaded.');
	});

	it('surfaces a restart fallback with the underlying error instead of pretending the reload worked', () => {
		const message = saveStatusMessage('agent', 'reload', false, {
			action: 'restart-fallback',
			healthy: true,
			error: 'settings reload did not confirm within 35s'
		});
		expect(message).toBe(
			'Changes saved. CometMind reload failed and restarted instead (settings reload did not confirm within 35s).'
		);
	});

	it('surfaces an unhealthy restart fallback distinctly from a healthy one', () => {
		const message = saveStatusMessage('agent', 'reload', false, {
			action: 'restart-fallback',
			healthy: false,
			error: 'boom'
		});
		expect(message).toBe(
			'Changes saved. CometMind reload failed and the restart did not come back up (boom). Check the CometMind log.'
		);
	});

	it('flags a reload/restart that did not come back healthy', () => {
		expect(
			saveStatusMessage('agent', 'reload', false, { action: 'reload', healthy: false })
		).toBe('Changes saved. CometMind reloaded but is not responding yet. Check the CometMind log.');
		expect(
			saveStatusMessage('agent', 'restart', false, { action: 'restart', healthy: false })
		).toBe('Changes saved. CometMind restarted but is not responding yet. Check the CometMind log.');
	});

	it('reports no runtime note when reload is explicitly null (no action requested)', () => {
		expect(saveStatusMessage('agent', 'none', false, null)).toBe('Changes saved.');
	});
});
