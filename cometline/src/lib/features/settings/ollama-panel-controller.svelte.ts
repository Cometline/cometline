import type { ProviderConfig } from '$lib/types';
import {
	isValidOllamaModelName,
	OLLAMA_DEFAULT_NATIVE_BASE,
	type OllamaCatalogEntry
} from '$lib/ollama/catalog';
import {
	cancelOllamaPull,
	checkOllamaHealth,
	listOllamaModels,
	pullOllamaModel,
	type OllamaHealthResult,
	type OllamaInstalledModel,
	type OllamaPullProgress
} from '$lib/ollama/client';

function isCancelledPull(err: unknown): boolean {
	return err instanceof Error && /cancelled/i.test(err.message);
}

export function createOllamaPanelController(deps: {
	getProvider: () => ProviderConfig;
	onUpdate: (patch: Partial<ProviderConfig>) => void;
}) {
	let health = $state<OllamaHealthResult | null>(null);
	let checking = $state(false);
	let installed = $state<OllamaInstalledModel[]>([]);
	let error = $state('');
	let pullingId = $state('');
	let pullProgress = $state<OllamaPullProgress | null>(null);
	let customModel = $state('');

	const installedNames = $derived(new Set(installed.map((m) => m.name)));

	async function refresh() {
		checking = true;
		error = '';
		try {
			const next = await checkOllamaHealth(
				deps.getProvider().baseURL || OLLAMA_DEFAULT_NATIVE_BASE
			);
			health = next;
			if (next.ok) {
				const listed = await listOllamaModels(next.baseURL);
				installed = listed.models;
				const names = listed.models.map((m) => m.name);
				const models = Array.from(new Set([...deps.getProvider().models, ...names]));
				deps.onUpdate({
					baseURL: next.baseURL,
					models,
					enabledModels: deps
						.getProvider()
						.enabledModels.filter((m) => models.includes(m))
				});
			} else {
				installed = [];
			}
		} catch (err) {
			error = err instanceof Error ? err.message : String(err);
			health = {
				ok: false,
				state: 'unreachable',
				baseURL: deps.getProvider().baseURL || OLLAMA_DEFAULT_NATIVE_BASE,
				error
			};
		} finally {
			checking = false;
		}
	}

	async function pullEntry(entry: OllamaCatalogEntry) {
		pullingId = entry.id;
		error = '';
		pullProgress = { model: entry.pullName, status: 'starting' };
		try {
			const result = await pullOllamaModel({
				baseURL: deps.getProvider().baseURL || OLLAMA_DEFAULT_NATIVE_BASE,
				catalogId: entry.id
			});
			installed = result.models;
			const names = result.models.map((m) => m.name);
			deps.onUpdate({
				models: Array.from(new Set([...deps.getProvider().models, ...names])),
				enabled: true
			});
			await refresh();
		} catch (err) {
			if (!isCancelledPull(err)) {
				error = err instanceof Error ? err.message : String(err);
			}
		} finally {
			pullingId = '';
			pullProgress = null;
		}
	}

	async function pullCustom() {
		const name = customModel.trim();
		if (!isValidOllamaModelName(name)) {
			error = 'Enter a valid Ollama model name (e.g. llama3.2:3b)';
			return;
		}
		pullingId = `custom:${name}`;
		error = '';
		pullProgress = { model: name, status: 'starting' };
		try {
			const result = await pullOllamaModel({
				baseURL: deps.getProvider().baseURL || OLLAMA_DEFAULT_NATIVE_BASE,
				modelName: name
			});
			installed = result.models;
			const names = result.models.map((m) => m.name);
			deps.onUpdate({
				models: Array.from(new Set([...deps.getProvider().models, ...names])),
				enabled: true
			});
			customModel = '';
			await refresh();
		} catch (err) {
			if (!isCancelledPull(err)) {
				error = err instanceof Error ? err.message : String(err);
			}
		} finally {
			pullingId = '';
			pullProgress = null;
		}
	}

	async function cancelPull() {
		await cancelOllamaPull();
	}

	function toggleModel(model: string) {
		const provider = deps.getProvider();
		const enabled = provider.enabledModels.includes(model)
			? provider.enabledModels.filter((m) => m !== model)
			: [...provider.enabledModels, model];
		deps.onUpdate({
			enabledModels: enabled,
			selectedModel: enabled[0] || '',
			enabled: enabled.length > 0 ? true : provider.enabled
		});
	}

	function handlePullProgress(payload: OllamaPullProgress) {
		if (pullingId) pullProgress = payload;
	}

	return {
		get health() {
			return health;
		},
		get checking() {
			return checking;
		},
		get installed() {
			return installed;
		},
		get installedNames() {
			return installedNames;
		},
		get error() {
			return error;
		},
		get pullingId() {
			return pullingId;
		},
		get pullProgress() {
			return pullProgress;
		},
		get customModel() {
			return customModel;
		},
		set customModel(value: string) {
			customModel = value;
		},
		refresh,
		pullEntry,
		pullCustom,
		cancelPull,
		toggleModel,
		handlePullProgress
	};
}
