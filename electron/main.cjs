const { app, BrowserWindow, dialog, ipcMain, protocol, net, shell } = require('electron');
const path = require('path');
const { pathToFileURL } = require('url');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const { autoUpdater } = require('electron-updater');

const COMETMIND_PORT = 7700;
// Custom scheme used to serve the packaged SvelteKit bundle. Loading the
// fallback index.html over file:// breaks because adapter-static emits
// absolute asset paths (/_app/immutable/...) that resolve against the
// filesystem root. Serving the bundle through a registered standard scheme
// makes those absolute paths resolve against the bundle root instead.
const APP_SCHEME = 'app';
const APP_HOST = 'bundle';
const APP_ORIGIN = `${APP_SCHEME}://${APP_HOST}`;
const HEALTH_URL = `http://127.0.0.1:${COMETMIND_PORT}/api/v1/health`;
const MAX_RETRIES = 50;
const POLL_MS = 100;
const UPDATE_CHECK_INTERVAL_MS = 4 * 60 * 60 * 1000;

function defaultAppearance() {
	return {
		heroComposer: {
			glowColor: '#72c0ff',
			ringColor: '#9ed8ff'
		}
	};
}

function defaultShortcuts() {
	return {
		toggleSidebar: { command: true, key: 'b' },
		openSettings: { command: true, key: ',' },
		newChat: { command: true, key: 't' },
		stopResponse: { command: true, key: 'c' },
		sendMessage: { key: 'Enter', shift: false },
		closeSettings: { key: 'Escape' },
		focusSearch: { command: true, key: 'f' },
		previousSession: { ctrl: true, meta: true, key: 'ArrowUp' },
		nextSession: { ctrl: true, meta: true, key: 'ArrowDown' },
		toggleWebPanel: { command: true, alt: true, key: 'b' }
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
				selectedModel: '',
				models: [...OPENCODE_GO_AVAILABLE_MODELS],
				enabledModels: []
			}
		],
		activeProviderId: 'opencode-go',
		appearance: defaultAppearance(),
		shortcuts: defaultShortcuts()
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
const VALID_PROVIDER_METHODS = ['openai-compatible', 'openai', 'anthropic', 'opencode-go'];
const BUILTIN_PROVIDER_NAMES = {
	'openai-compatible': 'OpenAI Compatible',
	anthropic: 'Anthropic',
	openai: 'OpenAI',
	'opencode-go': 'OpenCode Go'
};

// Must run before app `ready`. Marking the scheme standard + secure gives the
// loaded page a normal web origin (so history API routing, fetch, and module
// scripts behave like https) instead of the restricted file:// origin.
protocol.registerSchemesAsPrivileged([
	{
		scheme: APP_SCHEME,
		privileges: {
			standard: true,
			secure: true,
			supportFetchAPI: true,
			stream: true,
			codeCache: true
		}
	}
]);

let mainWindow = null;
let cometMindProcess = null;
let stoppingForQuit = false;
let stoppedForQuit = false;
let relaunchForUpdate = false;
let updateCheckTimer = null;
let windowButtonAnimationTimer = null;
let windowButtonPosition = { x: 16, y: 17 };

// Vertically center the native buttons on the sidebar search bar. The titlebar
// row now sits flush against the window top, so the search input center is:
// titlebar row top padding (10px) + half the 28px search input (14px) = 24px.
// A traffic light is ~14px tall, so y = 24 - 7 = 17 lines the centers up.
const WINDOW_BUTTON_OPEN_POSITION = { x: 16, y: 17 };
const WINDOW_BUTTON_CLOSED_POSITION = { x: 17, y: 17 };
const WINDOW_BUTTON_DEFAULT_DURATION = 240;
const sidebarChromeEase = cubicBezier(0.22, 1, 0.36, 1);

function cubicBezier(x1, y1, x2, y2) {
	function sampleCurveX(t) {
		return ((1 - 3 * x2 + 3 * x1) * t + (3 * x2 - 6 * x1)) * t * t + 3 * x1 * t;
	}

	function sampleCurveY(t) {
		return ((1 - 3 * y2 + 3 * y1) * t + (3 * y2 - 6 * y1)) * t * t + 3 * y1 * t;
	}

	function sampleCurveDerivativeX(t) {
		return (3 * (1 - 3 * x2 + 3 * x1) * t + 2 * (3 * x2 - 6 * x1)) * t + 3 * x1;
	}

	function solveCurveX(x) {
		let t = x;
		for (let i = 0; i < 8; i++) {
			const currentX = sampleCurveX(t) - x;
			if (Math.abs(currentX) < 0.000001) return t;
			const derivative = sampleCurveDerivativeX(t);
			if (Math.abs(derivative) < 0.000001) break;
			t -= currentX / derivative;
		}

		let start = 0;
		let end = 1;
		t = x;
		for (let i = 0; i < 12; i++) {
			const currentX = sampleCurveX(t);
			if (Math.abs(currentX - x) < 0.000001) return t;
			if (x > currentX) start = t;
			else end = t;
			t = (end - start) * 0.5 + start;
		}
		return t;
	}

	return (x) => {
		if (x <= 0) return 0;
		if (x >= 1) return 1;
		return sampleCurveY(solveCurveX(x));
	};
}

function setWindowButtonPosition(position) {
	if (
		process.platform !== 'darwin' ||
		!mainWindow ||
		typeof mainWindow.setWindowButtonPosition !== 'function'
	) {
		return;
	}
	const next = { x: Math.round(position.x), y: Math.round(position.y) };
	mainWindow.setWindowButtonPosition(next);
	windowButtonPosition = next;
}

function animateWindowButtons(payload) {
	if (process.platform !== 'darwin' || !mainWindow) return;

	const open = typeof payload?.open === 'boolean' ? payload.open : Boolean(payload);
	const target = open ? WINDOW_BUTTON_OPEN_POSITION : WINDOW_BUTTON_CLOSED_POSITION;
	const rawDuration = Number(payload?.duration);
	const duration = Number.isFinite(rawDuration)
		? Math.max(0, Math.min(rawDuration, 1000))
		: WINDOW_BUTTON_DEFAULT_DURATION;
	const start = { ...windowButtonPosition };

	if (windowButtonAnimationTimer) {
		clearTimeout(windowButtonAnimationTimer);
		windowButtonAnimationTimer = null;
	}

	if (duration <= 16 || (start.x === target.x && start.y === target.y)) {
		setWindowButtonPosition(target);
		return;
	}

	const startedAt = Date.now();
	const step = () => {
		if (!mainWindow) return;
		const progress = Math.min(1, (Date.now() - startedAt) / duration);
		const eased = sidebarChromeEase(progress);
		setWindowButtonPosition({
			x: start.x + (target.x - start.x) * eased,
			y: start.y + (target.y - start.y) * eased
		});

		if (progress < 1) {
			windowButtonAnimationTimer = setTimeout(step, 16);
		} else {
			windowButtonAnimationTimer = null;
			setWindowButtonPosition(target);
		}
	};
	step();
}

function sendFullScreenState() {
	if (!mainWindow || mainWindow.isDestroyed()) return;
	mainWindow.webContents.send('cometline:fullscreen-changed', mainWindow.isFullScreen());
}

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

function resolveSystemPromptPath() {
	if (process.env.COMETMIND_SYSTEM_PROMPT_PATH) {
		return path.resolve(process.env.COMETMIND_SYSTEM_PROMPT_PATH);
	}
	if (app.isPackaged) {
		return path.join(process.resourcesPath, 'SOUL.md');
	}
	return path.join(__dirname, '..', 'SOUL.md');
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
		provider?.selectedModel || fallback.selectedModel || ''
	).trim();
	const rawEnabledModels = Array.isArray(provider?.enabledModels)
		? provider.enabledModels
		: legacySelected
			? [legacySelected]
			: [];
	const enabledModels = rawEnabledModels
		.map((model) => String(model || '').trim())
		.filter((model) => model && modelList.includes(model));
	const id = String(
		provider?.id ||
			fallback.id ||
			`provider-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
	).trim();
	const builtInName = BUILTIN_PROVIDER_NAMES[id];

	return {
		id,
		name: builtInName ?? String(provider?.name || fallback.name || 'Provider').trim(),
		method,
		enabled:
			typeof provider?.enabled === 'boolean' ? provider.enabled : Boolean(fallback.enabled),
		baseURL: String(provider?.baseURL ?? fallback.baseURL ?? '').trim(),
		apiKey: String(provider?.apiKey ?? fallback.apiKey ?? '').trim(),
		selectedModel: enabledModels[0] || '',
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

function isLegacySessionNavBinding(binding) {
	if (binding?.command) {
		return binding.alt !== true && binding.shift !== true && binding.ctrl !== true;
	}
	return Boolean(binding?.ctrl && binding.meta === false);
}

function normalizeSessionNavBinding(id, binding, defaultBinding) {
	if (id !== 'previousSession' && id !== 'nextSession') {
		return binding ?? defaultBinding;
	}
	if (!binding) return { ...defaultBinding };
	if (isLegacySessionNavBinding(binding)) {
		return { ...defaultBinding };
	}
	return binding;
}

function normalizeToggleWebPanelBinding(binding, defaultBinding) {
	if (!binding) return { ...defaultBinding };
	if (binding.key === 'b' && binding.command && binding.alt !== true) {
		return { ...defaultBinding };
	}
	return binding;
}

function normalizeShortcuts(shortcuts) {
	const defaults = defaultShortcuts();
	if (!shortcuts || typeof shortcuts !== 'object') return defaults;
	const next = { ...defaults };
	for (const id of Object.keys(defaults)) {
		const saved = shortcuts[id];
		if (saved && typeof saved === 'object' && typeof saved.key === 'string') {
			const normalized = {
				key: saved.key,
				...(typeof saved.command === 'boolean' && { command: saved.command }),
				...(typeof saved.ctrl === 'boolean' && { ctrl: saved.ctrl }),
				...(typeof saved.meta === 'boolean' && { meta: saved.meta }),
				...(typeof saved.alt === 'boolean' && { alt: saved.alt }),
				...(typeof saved.shift === 'boolean' && { shift: saved.shift })
			};
			if (id === 'toggleWebPanel') {
				next[id] = normalizeToggleWebPanelBinding(normalized, defaults[id]);
				continue;
			}
			next[id] = normalizeSessionNavBinding(id, normalized, defaults[id]);
		} else {
			next[id] =
				id === 'toggleWebPanel'
					? normalizeToggleWebPanelBinding(undefined, defaults[id])
					: normalizeSessionNavBinding(id, undefined, defaults[id]);
		}
	}
	return next;
}

function shortcutKeyMatches(a, b) {
	return a === b || String(a).toLowerCase() === String(b).toLowerCase();
}

function matchesInputShortcut(input, binding) {
	if (input.type !== 'keyDown' || !binding?.key) return false;
	if (!shortcutKeyMatches(input.key, binding.key)) return false;

	const expectsCommand = binding.command ?? false;
	if (expectsCommand) {
		if (!(input.control || input.meta)) return false;
		if (binding.alt !== undefined ? binding.alt !== input.alt : input.alt) return false;
		if (binding.shift !== undefined ? binding.shift !== input.shift : input.shift) return false;
		return true;
	}

	if (binding.ctrl !== undefined && binding.ctrl !== input.control) return false;
	if (binding.meta !== undefined && binding.meta !== input.meta) return false;
	if (binding.alt !== undefined && binding.alt !== input.alt) return false;
	if (binding.shift !== undefined && binding.shift !== input.shift) return false;
	return true;
}

let shortcutCaptureActive = false;
let sessionNavigationSuspended = false;
let webPanelOpen = false;

function sendCloseWebPanel() {
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.webContents.send('cometline:close-web-panel');
	}
}

function sendToggleWebPanel() {
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.webContents.send('cometline:toggle-web-panel');
	}
}

function isDarwinCloseWindowShortcut(input) {
	return (
		process.platform === 'darwin' &&
		input.type === 'keyDown' &&
		input.meta &&
		!input.control &&
		!input.alt &&
		!input.shift &&
		input.key?.toLowerCase() === 'w'
	);
}

function handleDarwinCloseWindowShortcut(event, input) {
	if (!isDarwinCloseWindowShortcut(input)) return false;
	event.preventDefault();
	if (webPanelOpen) {
		sendCloseWebPanel();
	}
	return true;
}

function attachMainWindowShortcuts(webContents) {
	webContents.on('before-input-event', (event, input) => {
		if (handleDarwinCloseWindowShortcut(event, input)) {
			return;
		}

		if (shortcutCaptureActive || sessionNavigationSuspended) return;
		const shortcuts = readProviderSettings().shortcuts ?? defaultShortcuts();
		if (matchesInputShortcut(input, shortcuts.previousSession)) {
			event.preventDefault();
			webContents.send('cometline:navigate-session', 'prev');
			return;
		}
		if (matchesInputShortcut(input, shortcuts.nextSession)) {
			event.preventDefault();
			webContents.send('cometline:navigate-session', 'next');
		}
	});
}

function handleWebPanelGuestShortcuts(event, input) {
	if (handleDarwinCloseWindowShortcut(event, input)) {
		return true;
	}
	if (shortcutCaptureActive || sessionNavigationSuspended) return false;
	const shortcuts = readProviderSettings().shortcuts ?? defaultShortcuts();
	if (matchesInputShortcut(input, shortcuts.toggleWebPanel)) {
		event.preventDefault();
		sendToggleWebPanel();
		return true;
	}
	return false;
}

function attachWebviewPanelShortcuts(webContents) {
	webContents.on('before-input-event', (event, input) => {
		handleWebPanelGuestShortcuts(event, input);
	});
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
	base.shortcuts = normalizeShortcuts(saved.shortcuts ?? base.shortcuts);
	if (!base.activeProviderId || !base.providers.find((p) => p.id === base.activeProviderId)) {
		base.activeProviderId =
			base.providers.find((p) => p.enabled && p.enabledModels.length > 0)?.id ??
			base.providers[0].id;
	}

	// Allow env overrides for the active provider only. Apply provider first so
	// key/baseURL/model attach to the provider selected by COMETMIND_PROVIDER.
	if (fromEnv.activeProviderId) {
		const matched = base.providers.find((p) => p.id === fromEnv.activeProviderId.trim());
		if (matched) base.activeProviderId = matched.id;
	}
	const active = base.providers.find((p) => p.id === base.activeProviderId) ?? base.providers[0];
	if (fromEnv.baseURL) active.baseURL = fromEnv.baseURL.trim();
	if (fromEnv.apiKey) active.apiKey = fromEnv.apiKey.trim();
	if (fromEnv.selectedModel) {
		const model = fromEnv.selectedModel.trim();
		active.selectedModel = model;
		active.enabled = true;
		if (model && !active.models.includes(model)) active.models = [...active.models, model];
		if (model && !active.enabledModels.includes(model)) active.enabledModels = [model];
	}

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
		appearance: normalizeAppearance(settings.appearance ?? current.appearance),
		shortcuts: normalizeShortcuts(settings.shortcuts ?? current.shortcuts)
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
system_prompt_path = ${JSON.stringify(resolveSystemPromptPath())}

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
		COMETMIND_MODEL: active.enabledModels[0] || active.selectedModel || active.models[0] || '',
		COMETMIND_SYSTEM_PROMPT_PATH: resolveSystemPromptPath()
	};
	if (active.baseURL) env.COMETMIND_BASE_URL = active.baseURL;
	if (active.apiKey) env.COMETMIND_API_KEY = active.apiKey;
	return env;
}

function getWorkspaceStoragePath() {
	const dir = path.join(os.homedir(), '.cometmind');
	if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
	return path.join(dir, 'cometline-workspace.json');
}

function readStoredWorkspacePath() {
	try {
		const raw = fs.readFileSync(getWorkspaceStoragePath(), 'utf8');
		const parsed = JSON.parse(raw);
		const stored = String(parsed?.workspacePath || '').trim();
		if (stored && fs.existsSync(stored)) return path.resolve(stored);
	} catch {
		// Fall through to defaults.
	}
	return '';
}

function writeStoredWorkspacePath(workspacePath) {
	const clean = path.resolve(workspacePath);
	fs.writeFileSync(getWorkspaceStoragePath(), JSON.stringify({ workspacePath: clean }, null, 2));
	fs.chmodSync(getWorkspaceStoragePath(), 0o600);
	return clean;
}

function getDefaultWorkspacePath() {
	const defaultPath = path.join(os.homedir(), 'Cometline');
	if (!fs.existsSync(defaultPath)) {
		fs.mkdirSync(defaultPath, { recursive: true });
	}
	return defaultPath;
}

function getWorkspacePath() {
	if (process.env.COMETMIND_WORKSPACE_PATH) {
		return path.resolve(process.env.COMETMIND_WORKSPACE_PATH);
	}
	const stored = readStoredWorkspacePath();
	if (stored) return stored;
	return getDefaultWorkspacePath();
}

async function selectWorkspacePath() {
	const window = BrowserWindow.getFocusedWindow();
	const result = await dialog.showOpenDialog(window || undefined, {
		properties: ['openDirectory', 'createDirectory'],
		buttonLabel: 'Select workspace',
		title: 'Choose a workspace folder'
	});
	if (result.canceled || result.filePaths.length === 0) return null;
	return writeStoredWorkspacePath(result.filePaths[0]);
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

	cometMindProcess = spawn(
		binary,
		['serve', '--port', String(COMETMIND_PORT), '--watch-parent'],
		{
			stdio: ['ignore', 'pipe', 'pipe'],
			env: providerEnv()
		}
	);

	cometMindProcess.stdout.on('data', (data) => logStream.write(data));
	cometMindProcess.stderr.on('data', (data) => logStream.write(data));

	cometMindProcess.on('exit', (code) => {
		console.log(`CometMind exited with code ${code}`);
		logStream.end();
		cometMindProcess = null;
	});

	cometMindProcess.on('error', (err) => {
		console.error('CometMind spawn error:', err);
		logStream.end();
		cometMindProcess = null;
	});
}

function stopCometMind() {
	const proc = cometMindProcess;
	if (!proc) return Promise.resolve();
	cometMindProcess = null;

	return new Promise((resolve) => {
		let settled = false;
		let forceTimer = null;
		const finish = () => {
			if (settled) return;
			settled = true;
			if (forceTimer) clearTimeout(forceTimer);
			resolve();
		};

		// Wait for the process to actually exit so it releases the TCP port
		// (127.0.0.1:7700) and the SQLite WAL lock before a new `serve` spawns.
		// Spawning a replacement too early causes "address already in use" and
		// SQLITE_BUSY (database is locked) while both processes hold the DB.
		proc.once('exit', finish);

		// Escalate to SIGKILL if graceful shutdown stalls past the server's
		// 5s shutdown budget, then resolve once it is gone.
		forceTimer = setTimeout(() => {
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

// Tracks the latest known auto-update state so a freshly loaded renderer can
// query it via IPC instead of waiting for the next event.
let updateState = { status: 'idle' };

function setUpdateState(next) {
	updateState = { ...next, updatedAt: Date.now() };
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.webContents.send('cometline:update-state', updateState);
	}
}

function configureAutoUpdater() {
	if (!app.isPackaged) return;

	// We surface a button in the UI and let the user choose when to restart, so
	// download automatically but never install behind their back.
	autoUpdater.autoDownload = true;
	autoUpdater.autoInstallOnAppQuit = false;
	autoUpdater.logger = {
		info: (message) => console.log(`[auto-updater] ${message}`),
		warn: (message) => console.warn(`[auto-updater] ${message}`),
		error: (message) => console.error(`[auto-updater] ${message}`),
		debug: (message) => console.debug(`[auto-updater] ${message}`)
	};

	autoUpdater.on('checking-for-update', () => {
		setUpdateState({ status: 'checking' });
	});

	autoUpdater.on('update-available', (info) => {
		setUpdateState({ status: 'downloading', version: info?.version, percent: 0 });
	});

	autoUpdater.on('update-not-available', (info) => {
		setUpdateState({ status: 'idle', version: info?.version });
	});

	autoUpdater.on('download-progress', (progress) => {
		setUpdateState({
			status: 'downloading',
			percent: Math.round(progress?.percent ?? 0)
		});
	});

	autoUpdater.on('update-downloaded', (info) => {
		setUpdateState({ status: 'ready', version: info?.version });
	});

	autoUpdater.on('error', (err) => {
		console.error('Auto-update error:', err);
		setUpdateState({ status: 'error', message: String(err?.message ?? err) });
	});

	const check = () => {
		autoUpdater.checkForUpdates().catch((err) => {
			console.error('Auto-update check failed:', err);
		});
	};

	check();
	updateCheckTimer = setInterval(check, UPDATE_CHECK_INTERVAL_MS);
}

function getBundleDir() {
	return path.join(__dirname, '..', 'build');
}

// Serves the packaged SvelteKit bundle over the custom app:// scheme. Absolute
// asset paths (e.g. /_app/immutable/x.js) map onto the bundle root, and any
// request that does not resolve to a real file falls back to index.html so the
// SPA router can handle client-side routes on reload.
function registerAppProtocol() {
	const bundleDir = getBundleDir();
	const fallback = path.join(bundleDir, 'index.html');

	protocol.handle(APP_SCHEME, (request) => {
		const requestUrl = new URL(request.url);
		// Decode and strip the leading slash so it resolves within the bundle.
		let relativePath = decodeURIComponent(requestUrl.pathname).replace(/^\/+/, '');
		if (!relativePath) relativePath = 'index.html';

		let resolved = path.normalize(path.join(bundleDir, relativePath));

		// Guard against path traversal escaping the bundle directory.
		const withinBundle = resolved === bundleDir || resolved.startsWith(bundleDir + path.sep);
		if (!withinBundle) {
			return new Response('Forbidden', { status: 403 });
		}

		// Fall back to the SPA shell for client-side routes (no real file).
		if (!fs.existsSync(resolved) || !fs.statSync(resolved).isFile()) {
			resolved = fallback;
		}

		return net.fetch(pathToFileURL(resolved).toString());
	});
}

async function createWindow() {
	const icon = getAppIconPath();
	mainWindow = new BrowserWindow({
		width: 1200,
		height: 800,
		minWidth: 880,
		minHeight: 560,
		// 'hidden' (not 'hiddenInset') is required for setWindowButtonPosition to
		// take effect: Electron only honors custom traffic-light positions on a
		// frameless window. With 'hiddenInset' the buttons are pinned at a fixed
		// inset and every setWindowButtonPosition call is silently ignored.
		titleBarStyle: 'hidden',
		...(process.platform === 'darwin'
			? {
					backgroundColor: '#00000000',
					transparent: true,
					vibrancy: 'sidebar',
					visualEffectState: 'active'
				}
			: {}),
		...(icon ? { icon } : {}),
		show: false,
		webPreferences: {
			preload: path.join(__dirname, 'preload.cjs'),
			contextIsolation: true,
			nodeIntegration: false,
			allowRunningInsecureContent: false,
			webviewTag: true
		}
	});
	setWindowButtonPosition(WINDOW_BUTTON_OPEN_POSITION);
	if (process.platform === 'darwin' && icon) app.dock?.setIcon(icon);

	// Defense in depth: untrusted markdown links must never navigate the app
	// window or spawn Electron child windows. External links are routed through
	// the validated cometline:open-external IPC handler instead.
	mainWindow.webContents.setWindowOpenHandler(({ url }) => {
		if (isExternallyOpenableUrl(url)) void shell.openExternal(url);
		return { action: 'deny' };
	});
	mainWindow.webContents.on('will-navigate', (event, url) => {
		// Allow the app's own origin (dev server or app:// bundle); block the rest.
		const allowed = url.startsWith(`${APP_ORIGIN}/`) || url.startsWith('http://127.0.0.1:5173');
		if (!allowed) {
			event.preventDefault();
			if (isExternallyOpenableUrl(url)) void shell.openExternal(url);
		}
	});
	attachMainWindowShortcuts(mainWindow.webContents);
	mainWindow.webContents.on('did-attach-webview', (_event, webContents) => {
		attachWebviewPanelShortcuts(webContents);
	});

	if (app.isPackaged) {
		await mainWindow.loadURL(`${APP_ORIGIN}/`);
	} else {
		await mainWindow.loadURL('http://127.0.0.1:5173');
	}

	mainWindow.once('ready-to-show', () => {
		mainWindow.show();
		// Re-apply once the window is realized; an early call right after
		// construction can be dropped before the buttons exist.
		setWindowButtonPosition(WINDOW_BUTTON_OPEN_POSITION);
		sendFullScreenState();
	});

	// macOS hides the native traffic lights in fullscreen, freeing the gutter.
	// Tell the renderer so it can reclaim that space for the search bar.
	mainWindow.on('enter-full-screen', sendFullScreenState);
	mainWindow.on('leave-full-screen', sendFullScreenState);

	// On macOS, Cmd+W (and the red traffic light) should hide the window rather
	// than destroy it. Recreating the BrowserWindow on every reopen forces a
	// cold renderer boot — reloading the whole SvelteKit bundle and reconnecting
	// to CometMind — which feels slow. Hiding keeps the renderer warm so the
	// next Dock click re-shows instantly. We still allow a real close during
	// quit (Cmd+Q / before-quit), where stoppingForQuit is set.
	mainWindow.on('close', (event) => {
		if (process.platform === 'darwin' && !stoppingForQuit && !stoppedForQuit) {
			event.preventDefault();
			if (mainWindow.isFullScreen()) {
				// Leaving fullscreen first avoids a black Space lingering after hide.
				mainWindow.once('leave-full-screen', () => mainWindow?.hide());
				mainWindow.setFullScreen(false);
			} else {
				mainWindow.hide();
			}
		}
	});

	mainWindow.on('closed', () => {
		if (windowButtonAnimationTimer) {
			clearTimeout(windowButtonAnimationTimer);
			windowButtonAnimationTimer = null;
		}
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
	// Serve the packaged bundle over the custom app:// scheme.
	if (app.isPackaged) registerAppProtocol();

	// Ensure CometMind config exists before starting so the active provider is
	// available even on first launch.
	writeCometMindConfig(readProviderSettings());
	startCometMind();
	const healthy = await waitForHealth();
	if (!healthy) {
		console.error('CometMind failed to become healthy');
	}
	await createWindow();
	configureAutoUpdater();

	app.on('activate', () => {
		// Reopening from the Dock: re-show the warm, hidden window if it still
		// exists (instant); only build a fresh one if it was actually destroyed.
		if (mainWindow && !mainWindow.isDestroyed()) {
			mainWindow.show();
			mainWindow.focus();
		} else if (BrowserWindow.getAllWindows().length === 0) {
			createWindow();
		}
	});
});

app.on('window-all-closed', () => {
	if (process.platform !== 'darwin') {
		app.quit();
	}
});

app.on('before-quit', async (event) => {
	if (stoppedForQuit || stoppingForQuit) return;

	if (relaunchForUpdate) {
		// quitAndInstall() is driving the quit; don't intercept it so the
		// updater can relaunch the app after installation.
		if (updateCheckTimer) {
			clearInterval(updateCheckTimer);
			updateCheckTimer = null;
		}
		return;
	}

	event.preventDefault();
	stoppingForQuit = true;
	if (updateCheckTimer) {
		clearInterval(updateCheckTimer);
		updateCheckTimer = null;
	}
	await stopCometMind();
	stoppedForQuit = true;
	app.quit();
});

// Last-resort synchronous cleanup. `before-quit` handles graceful shutdown,
// but if the main process exits without it (e.g. an uncaught crash) this best
// effort SIGTERM avoids leaving an orphaned sidecar holding the port and DB
// lock. The Go-side --watch-parent watchdog is the real safety net for hard
// kills where no JS runs at all.
process.on('exit', () => {
	if (cometMindProcess) {
		try {
			cometMindProcess.kill('SIGTERM');
		} catch {
			// ignore
		}
	}
});

ipcMain.on('cometmind:restart', async () => {
	await stopCometMind();
	startCometMind();
});

ipcMain.on('cometline:shortcut-capture-active', (_event, active) => {
	shortcutCaptureActive = Boolean(active);
});

ipcMain.on('cometline:session-navigation-suspended', (_event, suspended) => {
	sessionNavigationSuspended = Boolean(suspended);
});

ipcMain.on('cometline:web-panel-open', (_event, open) => {
	webPanelOpen = Boolean(open);
});

ipcMain.on('cometline:set-sidebar-open', (_event, payload) => {
	animateWindowButtons(payload);
});

ipcMain.handle('cometline:get-fullscreen', () =>
	Boolean(mainWindow && !mainWindow.isDestroyed() && mainWindow.isFullScreen())
);

ipcMain.handle('cometline:get-workspace-path', () => getWorkspacePath());

ipcMain.handle('cometline:select-workspace-path', selectWorkspacePath);

ipcMain.handle('cometline:set-workspace-path', (_event, workspacePath) => {
	const clean = writeStoredWorkspacePath(workspacePath);
	return clean;
});

ipcMain.handle('cometline:get-provider-settings', () => readProviderSettings());

ipcMain.handle('cometline:fetch-provider-models', async (_event, config) => {
	return fetchProviderModels(config);
});

ipcMain.handle('cometline:save-provider-settings', async (_event, settings, options = {}) => {
	const saved = writeProviderSettings(settings);
	if (options.restartCometMind !== false) {
		await stopCometMind();
		startCometMind();
		void waitForHealth();
	}
	return saved;
});

// Opens a markdown link in the user's default browser. Only http(s)/mailto are
// allowed so a malicious link can't launch arbitrary local handlers.
function isExternallyOpenableUrl(rawUrl) {
	try {
		const parsed = new URL(String(rawUrl));
		return (
			parsed.protocol === 'http:' ||
			parsed.protocol === 'https:' ||
			parsed.protocol === 'mailto:'
		);
	} catch {
		return false;
	}
}

ipcMain.handle('cometline:open-external', async (_event, rawUrl) => {
	if (!isExternallyOpenableUrl(rawUrl)) return false;
	await shell.openExternal(String(rawUrl));
	return true;
});

ipcMain.handle('cometline:get-app-version', () => app.getVersion());

ipcMain.handle('cometline:get-update-state', () => updateState);

ipcMain.handle('cometline:check-for-updates', async () => {
	if (!app.isPackaged) return { status: 'idle' };
	try {
		await autoUpdater.checkForUpdates();
	} catch (err) {
		console.error('Manual update check failed:', err);
		setUpdateState({ status: 'error', message: String(err?.message ?? err) });
	}
	return updateState;
});

ipcMain.handle('cometline:install-update', async () => {
	if (updateState.status !== 'ready') return false;
	relaunchForUpdate = true;
	stoppingForQuit = true;
	// Stop the sidecar gracefully before the updater takes over the quit flow.
	await stopCometMind();
	// isSilent=true, isForceRunAfter=true so the updater relaunches the app.
	setImmediate(() => autoUpdater.quitAndInstall(true, true));
	return true;
});
