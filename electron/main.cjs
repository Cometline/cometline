const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');

const COMETMIND_PORT = 7700;
const HEALTH_URL = `http://127.0.0.1:${COMETMIND_PORT}/api/v1/health`;
const MAX_RETRIES = 50;
const POLL_MS = 100;

function defaultAppearance() {
	return {
		heroComposer: {
			glowColor: '#72c0ff',
			ringColor: '#9ed8ff'
		}
	};
}

function defaultProviderSettings() {
	return {
		providers: [
			{
				id: 'openai-compatible',
				name: 'OpenAI Compatible',
				method: 'openai-compatible',
				enabled: false,
				baseURL: '',
				apiKey: '',
				selectedModel: '',
				models: [],
				enabledModels: []
			},
			{
				id: 'anthropic',
				name: 'Anthropic',
				method: 'anthropic',
				enabled: false,
				baseURL: 'https://api.anthropic.com',
				apiKey: '',
				selectedModel: '',
				models: [],
				enabledModels: []
			},
			{
				id: 'openai',
				name: 'OpenAI',
				method: 'openai',
				enabled: false,
				baseURL: 'https://api.openai.com/v1',
				apiKey: '',
				selectedModel: '',
				models: [],
				enabledModels: []
			},
			{
				id: 'opencode-go',
				name: 'OpenCode Go',
				method: 'opencode-go',
				enabled: true,
				baseURL: 'https://opencode.ai/zen/go/v1',
				apiKey: '',
				selectedModel: DEFAULT_OPENCODE_GO_ENABLED_MODELS[0],
				models: [...OPENCODE_GO_AVAILABLE_MODELS],
				enabledModels: [...DEFAULT_OPENCODE_GO_ENABLED_MODELS]
			}
		],
		activeProviderId: 'opencode-go',
		appearance: defaultAppearance()
	};
}

const OPENCODE_GO_AVAILABLE_MODELS = [
	'deepseek-v4-flash',
	'deepseek-v4-pro',
	'glm-5',
	'glm-5.1',
	'kimi-k2.6',
	'kimi-k2.7-code',
	'mimo-v2.5',
	'mimo-v2.5-pro',
	'minimax-m2.7',
	'minimax-m3',
	'qwen3.6-plus',
	'qwen3.7-max',
	'qwen3.7-plus'
];
const DEFAULT_OPENCODE_GO_ENABLED_MODELS = ['deepseek-v4-flash'];
const VALID_PROVIDER_METHODS = ['openai-compatible', 'openai', 'anthropic', 'opencode-go'];

let mainWindow = null;
let cometMindProcess = null;

function resolveCometMindBinary() {
	if (process.env.COMETMIND_BINARY_PATH) {
		return process.env.COMETMIND_BINARY_PATH;
	}
	if (app.isPackaged) {
		return path.join(process.resourcesPath, 'cometmind');
	}
	// Dev: repository layout from cometline/electron/main.cjs
	const devCandidate = path.join(__dirname, '..', '..', 'cometmind', 'dist', 'cometmind');
	if (fs.existsSync(devCandidate)) return devCandidate;
	return path.join(__dirname, '..', '..', 'cometmind', 'cometmind');
}

function getLogPath() {
	const dir = path.join(os.homedir(), '.cometmind');
	if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
	return path.join(dir, 'cometline.log');
}

function getSettingsPath() {
	const dir = path.join(os.homedir(), '.cometmind');
	if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
	return path.join(dir, 'cometline-settings.json');
}

function getConfigPath() {
	const dir = path.join(os.homedir(), '.cometmind');
	if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
	return path.join(dir, 'config.toml');
}

function migrateSingleProvider(saved) {
	// Old format stored a single provider at the top level.
	if (saved && typeof saved === 'object' && !Array.isArray(saved.providers)) {
		const id = String(saved.provider || 'openai').trim();
		return {
			providers: [
				{
					id,
					name:
						id === 'opencode-go'
							? 'OpenCode Go'
							: id.charAt(0).toUpperCase() + id.slice(1),
					method:
						id === 'openai' && saved.baseURL?.includes('opencode.ai')
							? 'opencode-go'
							: id === 'openai'
								? 'openai-compatible'
								: id,
					enabled: true,
					baseURL: String(saved.baseURL || '').trim(),
					apiKey: String(saved.apiKey || '').trim(),
					selectedModel: String(saved.selectedModel || '').trim(),
					models: Array.isArray(saved.models) ? saved.models.filter(Boolean) : [],
					enabledModels: saved.selectedModel ? [String(saved.selectedModel).trim()] : []
				}
			],
			activeProviderId: id
		};
	}
	return null;
}

function normalizeProvider(provider, fallback = {}) {
	const method = VALID_PROVIDER_METHODS.includes(provider?.method)
		? provider.method
		: fallback.method || 'openai-compatible';
	const rawModels = Array.isArray(provider?.models) ? provider.models : fallback.models || [];
	const models = rawModels.map((model) => String(model || '').trim()).filter(Boolean);
	const modelList =
		method === 'opencode-go'
			? Array.from(new Set([...OPENCODE_GO_AVAILABLE_MODELS, ...models]))
			: models;
	const legacySelected = String(
		provider?.selectedModel || fallback.selectedModel || modelList[0] || ''
	).trim();
	const rawEnabledModels = Array.isArray(provider?.enabledModels)
		? provider.enabledModels
		: legacySelected
			? [legacySelected]
			: [];
	const enabledModels = rawEnabledModels
		.map((model) => String(model || '').trim())
		.filter((model) => model && modelList.includes(model));
	return {
		id: String(
			provider?.id ||
				fallback.id ||
				`provider-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
		).trim(),
		name: String(provider?.name || fallback.name || 'Provider').trim(),
		method,
		enabled:
			typeof provider?.enabled === 'boolean' ? provider.enabled : Boolean(fallback.enabled),
		baseURL: String(provider?.baseURL ?? fallback.baseURL ?? '').trim(),
		apiKey: String(provider?.apiKey ?? fallback.apiKey ?? '').trim(),
		selectedModel:
			enabledModels[0] ||
			(modelList.includes(legacySelected) ? legacySelected : modelList[0] || ''),
		models: modelList,
		enabledModels
	};
}

function normalizeHexColor(value, fallback) {
	if (typeof value !== 'string') return fallback;
	const trimmed = value.trim();
	if (!/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(trimmed)) return fallback;
	if (trimmed.length === 4) {
		return `#${trimmed[1]}${trimmed[1]}${trimmed[2]}${trimmed[2]}${trimmed[3]}${trimmed[3]}`.toLowerCase();
	}
	return trimmed.toLowerCase();
}

function normalizeAppearance(appearance) {
	const defaults = defaultAppearance();
	return {
		heroComposer: {
			glowColor: normalizeHexColor(
				appearance?.heroComposer?.glowColor,
				defaults.heroComposer.glowColor
			),
			ringColor: normalizeHexColor(
				appearance?.heroComposer?.ringColor,
				defaults.heroComposer.ringColor
			)
		}
	};
}

function normalizeProviders(providers) {
	const defaults = defaultProviderSettings().providers;
	const saved = Array.isArray(providers) ? providers : [];
	const defaultProviders = defaults.map((provider) => {
		const savedProvider = saved.find((p) => p.id === provider.id);
		return normalizeProvider(savedProvider || provider, provider);
	});
	const customProviders = saved
		.filter((provider) => !defaults.some((p) => p.id === provider.id))
		.map((provider) => normalizeProvider(provider));
	return [...defaultProviders, ...customProviders];
}

function readProviderSettings() {
	const defaults = defaultProviderSettings();
	const fromEnv = {
		activeProviderId: process.env.COMETMIND_PROVIDER,
		baseURL: process.env.COMETMIND_BASE_URL,
		apiKey:
			process.env.COMETMIND_API_KEY ||
			process.env.OPENAI_API_KEY ||
			process.env.ANTHROPIC_API_KEY,
		selectedModel: process.env.COMETMIND_MODEL
	};

	let saved = {};
	const settingsPath = getSettingsPath();
	if (fs.existsSync(settingsPath)) {
		try {
			saved = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
		} catch {
			saved = {};
		}
	}

	const migrated = migrateSingleProvider(saved);
	const base = migrated ? migrated : { ...defaults, ...saved };

	base.providers = normalizeProviders(base.providers);
	base.appearance = normalizeAppearance(saved.appearance ?? base.appearance);
	if (!base.activeProviderId || !base.providers.find((p) => p.id === base.activeProviderId)) {
		base.activeProviderId =
			base.providers.find((p) => p.enabled && p.enabledModels.length > 0)?.id ??
			base.providers[0].id;
	}

	// Allow env overrides for the active provider only.
	const active = base.providers.find((p) => p.id === base.activeProviderId) ?? base.providers[0];
	if (fromEnv.activeProviderId) {
		const matched = base.providers.find((p) => p.id === fromEnv.activeProviderId.trim());
		if (matched) base.activeProviderId = matched.id;
	}
	if (fromEnv.baseURL) active.baseURL = fromEnv.baseURL.trim();
	if (fromEnv.apiKey) active.apiKey = fromEnv.apiKey.trim();
	if (fromEnv.selectedModel) active.selectedModel = fromEnv.selectedModel.trim();

	return base;
}

function writeProviderSettings(settings) {
	const current = readProviderSettings();
	const nextProviders = Array.isArray(settings.providers)
		? normalizeProviders(settings.providers)
		: current.providers;
	const requestedActive = nextProviders.find((p) => p.id === settings.activeProviderId);
	const nextActive =
		requestedActive?.id ??
		nextProviders.find((p) => p.enabled && p.enabledModels.length > 0)?.id ??
		nextProviders.find((p) => p.enabled)?.id ??
		nextProviders[0]?.id ??
		'';
	const next = {
		providers: nextProviders,
		activeProviderId: nextActive,
		appearance: normalizeAppearance(settings.appearance ?? current.appearance)
	};
	const settingsPath = getSettingsPath();
	fs.writeFileSync(settingsPath, JSON.stringify(next, null, 2));
	try {
		fs.chmodSync(settingsPath, 0o600);
	} catch {
		/* ignore */
	}
	writeCometMindConfig(next);
	return next;
}

function writeCometMindConfig(settings) {
	const runtimeProviders = settings.providers.filter(
		(p) => p.enabled && p.enabledModels.length > 0
	);
	const active =
		runtimeProviders.find((p) => p.id === settings.activeProviderId) ?? runtimeProviders[0];
	if (!active) return;

	const providerEntries = runtimeProviders
		.map(
			(p) => `[[providers]]
id = ${JSON.stringify(p.id)}
name = ${JSON.stringify(p.name)}
method = ${JSON.stringify(p.method)}
base_url = ${JSON.stringify(p.baseURL)}
api_key = ${JSON.stringify(p.apiKey)}
model = ${JSON.stringify(p.enabledModels[0] || p.selectedModel || p.models[0] || '')}
`
		)
		.join('\n');

	const content = `# CometMind — generated by Cometline
provider = ${JSON.stringify(active.id)}
model = ${JSON.stringify(active.enabledModels[0] || active.selectedModel || active.models[0] || '')}
base_url = ${JSON.stringify(active.baseURL)}
max_tokens = 8192
max_steps = 50

${providerEntries}`;

	const configPath = getConfigPath();
	fs.writeFileSync(configPath, content);
	try {
		fs.chmodSync(configPath, 0o600);
	} catch {
		/* ignore */
	}
}

function providerEnv() {
	const settings = readProviderSettings();
	const runtimeProviders = settings.providers.filter(
		(p) => p.enabled && p.enabledModels.length > 0
	);
	const active =
		runtimeProviders.find((p) => p.id === settings.activeProviderId) ??
		runtimeProviders[0] ??
		settings.providers[0];
	const env = {
		...process.env,
		COMETMIND_PROVIDER: active.id,
		COMETMIND_MODEL: active.enabledModels[0] || active.selectedModel || active.models[0] || ''
	};
	if (active.baseURL) env.COMETMIND_BASE_URL = active.baseURL;
	if (active.apiKey) env.COMETMIND_API_KEY = active.apiKey;
	return env;
}

function getWorkspacePath() {
	if (process.env.COMETMIND_WORKSPACE_PATH) {
		return path.resolve(process.env.COMETMIND_WORKSPACE_PATH);
	}
	if (app.isPackaged) {
		return os.homedir();
	}
	return path.resolve(__dirname, '..', '..');
}

function getAppIconPath() {
	const candidates = app.isPackaged
		? [path.join(process.resourcesPath, 'icon.png')]
		: [path.join(__dirname, '..', 'buildResources', 'icon.png')];
	return candidates.find((candidate) => fs.existsSync(candidate));
}

function startCometMind() {
	if (cometMindProcess) return;

	const binary = resolveCometMindBinary();
	const logPath = getLogPath();
	const logStream = fs.createWriteStream(logPath, { flags: 'a' });

	if (!fs.existsSync(binary)) {
		console.error(`CometMind binary not found: ${binary}`);
		return;
	}

	cometMindProcess = spawn(binary, ['serve', '--port', String(COMETMIND_PORT)], {
		stdio: ['ignore', 'pipe', 'pipe'],
		env: providerEnv()
	});

	cometMindProcess.stdout.on('data', (data) => logStream.write(data));
	cometMindProcess.stderr.on('data', (data) => logStream.write(data));

	cometMindProcess.on('exit', (code) => {
		console.log(`CometMind exited with code ${code}`);
		cometMindProcess = null;
	});

	cometMindProcess.on('error', (err) => {
		console.error('CometMind spawn error:', err);
		cometMindProcess = null;
	});
}

function stopCometMind() {
	const proc = cometMindProcess;
	if (!proc) return Promise.resolve();
	cometMindProcess = null;

	return new Promise((resolve) => {
		let settled = false;
		const finish = () => {
			if (settled) return;
			settled = true;
			clearTimeout(forceTimer);
			resolve();
		};

		// Wait for the process to actually exit so it releases the TCP port
		// (127.0.0.1:7700) and the SQLite WAL lock before a new `serve` spawns.
		// Spawning a replacement too early causes "address already in use" and
		// SQLITE_BUSY (database is locked) while both processes hold the DB.
		proc.once('exit', finish);

		// Escalate to SIGKILL if graceful shutdown stalls past the server's
		// 5s shutdown budget, then resolve once it is gone.
		const forceTimer = setTimeout(() => {
			try {
				proc.kill('SIGKILL');
			} catch {
				// ignore
			}
			finish();
		}, 6000);

		try {
			proc.kill('SIGTERM');
		} catch {
			finish();
		}
	});
}

async function waitForHealth() {
	for (let i = 0; i < MAX_RETRIES; i++) {
		try {
			const res = await fetch(HEALTH_URL, { signal: AbortSignal.timeout(1000) });
			if (res.ok) return true;
		} catch {
			// keep polling
		}
		await new Promise((resolve) => setTimeout(resolve, POLL_MS));
	}
	return false;
}

async function createWindow() {
	const icon = getAppIconPath();
	mainWindow = new BrowserWindow({
		width: 1200,
		height: 800,
		minWidth: 880,
		minHeight: 560,
		titleBarStyle: 'hiddenInset',
		...(icon ? { icon } : {}),
		show: false,
		webPreferences: {
			preload: path.join(__dirname, 'preload.cjs'),
			contextIsolation: true,
			nodeIntegration: false,
			allowRunningInsecureContent: false
		}
	});
	if (process.platform === 'darwin' && icon) app.dock?.setIcon(icon);

	if (app.isPackaged) {
		mainWindow.loadFile(path.join(__dirname, '..', 'build', 'index.html'));
	} else {
		await mainWindow.loadURL('http://127.0.0.1:5173');
	}

	mainWindow.once('ready-to-show', () => {
		mainWindow.show();
	});

	mainWindow.on('closed', () => {
		mainWindow = null;
	});
}

function normalizeModelsBaseURL(rawBaseURL) {
	let baseURL = String(rawBaseURL || '').trim();
	if (!baseURL) throw new Error('Base URL is required');
	baseURL = baseURL.replace(/\/+$/, '');
	baseURL = baseURL.replace(/\/chat\/completions$/i, '');
	return `${baseURL}/models`;
}

function normalizeAnthropicModelsURL(rawBaseURL) {
	let baseURL = String(rawBaseURL || '').trim();
	if (!baseURL) throw new Error('Base URL is required');
	baseURL = baseURL.replace(/\/+$/, '');
	return `${baseURL}/v1/models`;
}

async function fetchOpenAIModels(baseURL, apiKey) {
	const url = normalizeModelsBaseURL(baseURL);
	const res = await fetch(url, {
		headers: {
			Authorization: `Bearer ${apiKey}`,
			Accept: 'application/json'
		},
		signal: AbortSignal.timeout(12000)
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	const payload = await res.json();
	const rawModels = Array.isArray(payload?.data)
		? payload.data
		: Array.isArray(payload)
			? payload
			: [];
	const models = rawModels
		.map((item) => (typeof item === 'string' ? item : item?.id))
		.filter((id) => typeof id === 'string' && id.trim())
		.map((id) => id.trim());
	if (models.length === 0) throw new Error('No models returned by provider');
	return Array.from(new Set(models)).sort();
}

async function fetchAnthropicModels(baseURL, apiKey) {
	const url = normalizeAnthropicModelsURL(baseURL);
	const res = await fetch(url, {
		headers: {
			'x-api-key': apiKey,
			'anthropic-version': '2023-06-01',
			Accept: 'application/json'
		},
		signal: AbortSignal.timeout(12000)
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	const payload = await res.json();
	const rawModels = Array.isArray(payload?.data) ? payload.data : [];
	const models = rawModels
		.map((item) => (typeof item === 'string' ? item : item?.id))
		.filter((id) => typeof id === 'string' && id.trim())
		.map((id) => id.trim());
	if (models.length === 0) throw new Error('No models returned by Anthropic');
	return Array.from(new Set(models)).sort();
}

async function fetchProviderModels(config) {
	const method = config.method;
	if (method === 'opencode-go') {
		return [...OPENCODE_GO_AVAILABLE_MODELS];
	}

	const baseURL = String(config.baseURL || '').trim();
	const apiKey = String(config.apiKey || '').trim();
	if (!baseURL) throw new Error('Base URL is required');
	if (!apiKey) throw new Error('API key is required');

	if (method === 'anthropic') {
		return fetchAnthropicModels(baseURL, apiKey);
	}
	return fetchOpenAIModels(baseURL, apiKey);
}

app.whenReady().then(async () => {
	// Ensure CometMind config exists before starting so the active provider is
	// available even on first launch.
	writeCometMindConfig(readProviderSettings());
	startCometMind();
	const healthy = await waitForHealth();
	if (!healthy) {
		console.error('CometMind failed to become healthy');
	}
	await createWindow();

	app.on('activate', () => {
		if (BrowserWindow.getAllWindows().length === 0) createWindow();
	});
});

app.on('window-all-closed', () => {
	if (process.platform !== 'darwin') {
		stopCometMind();
		app.quit();
	}
});

app.on('before-quit', () => {
	stopCometMind();
});

ipcMain.on('cometmind:restart', async () => {
	await stopCometMind();
	startCometMind();
});

ipcMain.handle('cometline:get-workspace-path', () => getWorkspacePath());

ipcMain.handle('cometline:get-provider-settings', () => readProviderSettings());

ipcMain.handle('cometline:fetch-provider-models', async (_event, config) => {
	return fetchProviderModels(config);
});

ipcMain.handle('cometline:save-provider-settings', async (_event, settings) => {
	const saved = writeProviderSettings(settings);
	await stopCometMind();
	startCometMind();
	void waitForHealth();
	return saved;
});
