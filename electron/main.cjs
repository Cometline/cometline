const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');

const COMETMIND_PORT = 7700;
const HEALTH_URL = `http://127.0.0.1:${COMETMIND_PORT}/api/v1/health`;
const MAX_RETRIES = 50;
const POLL_MS = 100;
const DEFAULT_PROVIDER_SETTINGS = {
	provider: 'openai',
	baseURL: 'https://opencode.ai/zen/go/v1',
	apiKey: '',
	selectedModel: 'deepseek-v4-flash',
	models: ['deepseek-v4-flash']
};

let mainWindow = null;
let cometMindProcess = null;
let isQuitting = false;

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

function readProviderSettings() {
	const fromEnv = {
		provider: process.env.COMETMIND_PROVIDER,
		baseURL: process.env.COMETMIND_BASE_URL,
		apiKey: process.env.COMETMIND_API_KEY || process.env.OPENAI_API_KEY || process.env.ANTHROPIC_API_KEY,
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

	const merged = { ...DEFAULT_PROVIDER_SETTINGS, ...saved };
	for (const [key, value] of Object.entries(fromEnv)) {
		if (typeof value === 'string' && value.trim()) merged[key] = value.trim();
	}
	if (!Array.isArray(merged.models) || merged.models.length === 0) {
		merged.models = [merged.selectedModel].filter(Boolean);
	}
	return merged;
}

function writeProviderSettings(settings) {
	const current = readProviderSettings();
	const next = {
		...current,
		provider: String(settings.provider || current.provider || 'openai').trim(),
		baseURL: String(settings.baseURL || '').trim(),
		apiKey: String(settings.apiKey || '').trim(),
		selectedModel: String(settings.selectedModel || current.selectedModel || '').trim(),
		models: Array.isArray(settings.models) ? settings.models.filter(Boolean) : current.models
	};
	if (!next.selectedModel && next.models.length > 0) next.selectedModel = next.models[0];
	fs.writeFileSync(getSettingsPath(), JSON.stringify(next, null, 2));
	return next;
}

function providerEnv() {
	const settings = readProviderSettings();
	const env = {
		...process.env,
		COMETMIND_PROVIDER: settings.provider,
		COMETMIND_MODEL: settings.selectedModel
	};
	if (settings.baseURL) env.COMETMIND_BASE_URL = settings.baseURL;
	if (settings.apiKey) env.COMETMIND_API_KEY = settings.apiKey;
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

async function fetchProviderModels(settings) {
	const url = normalizeModelsBaseURL(settings.baseURL);
	const apiKey = String(settings.apiKey || '').trim();
	if (!apiKey) throw new Error('API key is required');

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
	const rawModels = Array.isArray(payload?.data) ? payload.data : Array.isArray(payload) ? payload : [];
	const models = rawModels
		.map((item) => (typeof item === 'string' ? item : item?.id))
		.filter((id) => typeof id === 'string' && id.trim())
		.map((id) => id.trim());
	if (models.length === 0) throw new Error('No models returned by provider');
	return Array.from(new Set(models)).sort();
}

app.whenReady().then(async () => {
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
		isQuitting = true;
		stopCometMind();
		app.quit();
	}
});

app.on('before-quit', () => {
	isQuitting = true;
	stopCometMind();
});

ipcMain.on('cometmind:restart', async () => {
	await stopCometMind();
	startCometMind();
});

ipcMain.handle('cometline:get-workspace-path', () => getWorkspacePath());

ipcMain.handle('cometline:get-provider-settings', () => readProviderSettings());

ipcMain.handle('cometline:fetch-provider-models', async (_event, settings) => {
	return fetchProviderModels(settings);
});

ipcMain.handle('cometline:save-provider-settings', async (_event, settings) => {
	const saved = writeProviderSettings(settings);
	await stopCometMind();
	startCometMind();
	void waitForHealth();
	return saved;
});
