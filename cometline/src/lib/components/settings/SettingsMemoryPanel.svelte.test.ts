// @vitest-environment jsdom
//
// Regression test for the "Save changes stays disabled after picking an
// embedding model" bug. The Settings save button is gated by a `$derived`
// (`saveDisabled`) that reads this panel's `isDirty()`. If `isDirty()` (or any
// helper it calls) mutates reactive `$state`, Svelte 5 dependency tracking
// breaks and selecting an embedding model no longer flips dirtiness.
//
// This test mounts the REAL panel (the controller test stubs `isDirty`, which is
// exactly how the original bug slipped through) and asserts that changing the
// embedding dropdown makes `isDirty()` return true — without first calling
// `buildSavePayload()` and without `isDirty()` mutating state.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';
import { flushSync } from 'svelte';
import type { ProviderConfig } from '$lib/types';
import SettingsMemoryPanel from './SettingsMemoryPanel.svelte';
import { createSettingsController } from './settings-controller.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';

vi.mock('$lib/client/cometmind', () => {
	const base = {
		enabled: true,
		auto_extract: true,
		auto_retrieve: true,
		max_retrieved: 5,
		similarity_threshold: 0.5,
		extraction_model: '',
		lifecycle: {
			decay_half_life_days: 30,
			forget_threshold: 0.1,
			usage_boost_factor: 0.15,
			max_usage_boost: 2,
			max_memories: 500,
			compaction_target_ratio: 0.8,
			compaction_on_extract: true
		},
		embedding: { provider_id: '', provider: '', model: '', base_url: '', api_key: '' }
	};
	return {
		getMemorySettings: () => Promise.resolve(structuredClone(base)),
		putMemorySettings: (s: unknown) => Promise.resolve(structuredClone(s)),
		listMemories: () => Promise.resolve({ memories: [] }),
		searchMemories: vi.fn(),
		createMemory: vi.fn(),
		deleteMemory: vi.fn(),
		compactMemory: vi.fn(),
		compactMemoryPreview: vi.fn(),
		defaultMemorySettings: () => structuredClone(base)
	};
});

const providers: ProviderConfig[] = [
	{
		id: 'openai-1',
		name: 'OpenAI',
		method: 'openai',
		enabled: true,
		baseURL: '',
		apiKey: 'sk-test',
		selectedModel: 'text-embedding-3-small',
		models: ['text-embedding-3-small', 'text-embedding-3-large'],
		enabledModels: ['text-embedding-3-small', 'text-embedding-3-large']
	}
];

afterEach(() => {
	vi.clearAllMocks();
});

function selectModel(select: HTMLSelectElement, value: string) {
	select.value = value;
	select.dispatchEvent(new Event('change', { bubbles: true }));
}

describe('SettingsMemoryPanel embedding selection', () => {
	it('flips isDirty() to true when an embedding model is selected', async () => {
		const { component, container } = render(SettingsMemoryPanel, { props: { providers } });

		// Wait for the async reload() to render the dropdown with its options.
		let select!: HTMLSelectElement;
		await waitFor(() => {
			select = container.querySelector('select') as HTMLSelectElement;
			expect(select).toBeTruthy();
			expect(select.options.length).toBeGreaterThan(1);
		});

		// Initially nothing selected => not dirty.
		expect(component.isDirty()).toBe(false);

		// Pick the first real embedding model.
		selectModel(select, 'openai-1:text-embedding-3-small');

		// The bug: isDirty() stayed false because the dirty path mutated state
		// inside a reactive read and broke tracking. It must now be true — and
		// crucially WITHOUT having called buildSavePayload() first.
		await waitFor(() => expect(component.isDirty()).toBe(true));
	});

	it('isDirty() is a pure read and does not mutate panel state', async () => {
		const { component, container } = render(SettingsMemoryPanel, { props: { providers } });

		let select!: HTMLSelectElement;
		await waitFor(() => {
			select = container.querySelector('select') as HTMLSelectElement;
			expect(select).toBeTruthy();
			expect(select.options.length).toBeGreaterThan(1);
		});

		selectModel(select, 'openai-1:text-embedding-3-large');
		await waitFor(() => expect(component.isDirty()).toBe(true));

		// Calling isDirty() repeatedly must be stable (no hidden state writes).
		expect(component.isDirty()).toBe(true);
		expect(component.isDirty()).toBe(true);

		// And the eventual save payload reflects the selection.
		const payload = component.buildSavePayload();
		expect(payload.embedding.model).toBe('text-embedding-3-large');
	});

	// This is the test that actually reproduces the production bug: the Save
	// button's `disabled` is a `$derived` (`saveDisabled`) that reads the panel's
	// `isDirty()` exactly like SettingsPanel.svelte does. A direct `isDirty()`
	// call works even with the buggy (state-mutating) implementation; the failure
	// only surfaces when `isDirty()` is read from inside a reactive `$derived`.
	it('enables the controller save button when an embedding model is selected', async () => {
		const { component, container } = render(SettingsMemoryPanel, { props: { providers } });

		let select!: HTMLSelectElement;
		await waitFor(() => {
			select = container.querySelector('select') as HTMLSelectElement;
			expect(select).toBeTruthy();
			expect(select.options.length).toBeGreaterThan(1);
		});

		const cleanup = $effect.root(() => {
			// draft === persisted, so only the memory panel can make things dirty.
			const controller = createSettingsController({
				getDraft: () => settingsStore.settings,
				getMemoryPanelDirty: () => component.isDirty(),
				getMemoryPanelBusy: () => component.isBusy()
			});
			controller.activeSection = 'memory';

			let observed = controller.saveDisabled;
			$effect(() => {
				observed = controller.saveDisabled;
			});
			flushSync();
			expect(observed).toBe(true); // nothing selected yet

			selectModel(select, 'openai-1:text-embedding-3-small');
			flushSync();

			// Before the fix, the state mutation inside isDirty() broke dependency
			// tracking and `saveDisabled` stayed true here.
			expect(controller.hasPendingChanges).toBe(true);
			expect(observed).toBe(false);
		});
		cleanup();
	});
});
