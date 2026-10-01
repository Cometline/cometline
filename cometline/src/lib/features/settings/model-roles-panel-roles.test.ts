import { describe, expect, it } from 'vitest';
import { defaultCometMindSettings } from '$lib/cometmind-settings';
import type { ProviderConfig } from '$lib/types';
import {
	providerOptionLabel,
	withAutonomyProvider,
	withExtractionProvider,
	withTitleProvider
} from './model-roles-panel-roles';

const runtime = [
	{ id: 'a', name: 'Alpha', method: 'openai', enabledModels: ['a-1', 'a-2'] },
	{ id: 'local', name: 'Ollama', method: 'ollama', enabledModels: ['llama'] }
] as ProviderConfig[];

describe('model roles panel role updates', () => {
	it('pins the first model of the chosen provider and clears on empty id', () => {
		const pinned = withTitleProvider(defaultCometMindSettings(), runtime, 'local');
		expect([pinned.titleProviderId, pinned.titleModelId]).toEqual(['local', 'llama']);
		const cleared = withTitleProvider(pinned, runtime, '');
		expect([cleared.titleProviderId, cleared.titleModelId]).toEqual(['', '']);
	});

	it('keeps unknown extraction providers without a model', () => {
		const next = withExtractionProvider(defaultCometMindSettings(), runtime, 'gone');
		expect(next.memory.extractionProviderId).toBe('gone');
		expect(next.memory.extractionModel).toBe('');
	});

	it('falls back to the first runtime provider for autonomy', () => {
		const next = withAutonomyProvider(defaultCometMindSettings(), runtime, 'gone');
		expect([next.autonomy.providerId, next.autonomy.modelId]).toEqual(['a', 'a-1']);
	});

	it('labels local providers', () => {
		expect(providerOptionLabel(runtime[1])).toBe('Ollama (Local)');
		expect(providerOptionLabel(runtime[0])).toBe('Alpha');
	});
});
