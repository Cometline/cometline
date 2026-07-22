import { spawn, type ChildProcess } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import type { RuntimeContext } from './runtime-context.js';

const COMETMIND_PORT = 7700;
const HEALTH_URL = `http://127.0.0.1:${COMETMIND_PORT}/api/v1/health`;
const MAX_RETRIES = 50;
const POLL_MS = 100;
const RESPAWN_BASE_MS = 500;
const RESPAWN_MAX_MS = 10_000;
const RESPAWN_MAX_ATTEMPTS = 10;
const LOG_MAX_BYTES = 10 * 1024 * 1024;
const LOG_BACKUP_COUNT = 1;
const LOG_ROTATE_CHECK_BYTES = 512 * 1024;

export interface CometMindLifecycleDeps {
	context: RuntimeContext;
	resolveBinary(): string;
	providerEnv(): NodeJS.ProcessEnv;
	getLogPath(): string;
	getGatewayLogPath(): string;
}

export interface CometMindLifecycle {
	installCliShim(): void;
	start(): void;
	stop(): Promise<void>;
	reload(): Promise<{ action: string; healthy: boolean; error?: string }>;
	waitForHealth(): Promise<boolean>;
	syncDiscordGateway(settings: unknown): Promise<void>;
	isGatewayRunning(): boolean;
	terminateForExit(): void;
}

function rotateLogIfNeeded(logPath: string) {
	try {
		if (!fs.existsSync(logPath)) return;
		if (fs.statSync(logPath).size < LOG_MAX_BYTES) return;
		const oldest = `${logPath}.${LOG_BACKUP_COUNT}`;
		if (fs.existsSync(oldest)) fs.unlinkSync(oldest);
		fs.renameSync(logPath, oldest);
	} catch (error) {
		console.error(`Failed to rotate log ${logPath}:`, error);
	}
}

function createRotatingLogWriter(logPath: string) {
	rotateLogIfNeeded(logPath);
	let stream = fs.createWriteStream(logPath, { flags: 'a' });
	let bytesSinceCheck = 0;

	function maybeRotate() {
		try {
			if (!fs.existsSync(logPath) || fs.statSync(logPath).size < LOG_MAX_BYTES) return;
			stream.end();
			rotateLogIfNeeded(logPath);
			stream = fs.createWriteStream(logPath, { flags: 'a' });
			bytesSinceCheck = 0;
		} catch (error) {
			console.error(`Failed to rotate log ${logPath}:`, error);
		}
	}

	return {
		write(data: Buffer) {
			stream.write(data);
			bytesSinceCheck += data.length;
			if (bytesSinceCheck >= LOG_ROTATE_CHECK_BYTES) {
				bytesSinceCheck = 0;
				maybeRotate();
			}
		},
		end() {
			stream.end();
		}
	};
}

export function createCometMindLifecycle(deps: CometMindLifecycleDeps): CometMindLifecycle {
	let process: ChildProcess | null = null;
	let gatewayProcess: ChildProcess | null = null;
	let respawnTimer: NodeJS.Timeout | null = null;
	let respawnAttempts = 0;

	function cliBinDirs() {
		const home = os.homedir();
		const dirs = [path.join(home, '.cometmind', 'bin'), path.join(home, '.local', 'bin')];
		if (globalThis.process.platform === 'darwin') dirs.push('/opt/homebrew/bin', '/usr/local/bin');
		return dirs;
	}

	function pathWithCliBins(envPath = '') {
		const entries = [...cliBinDirs(), ...String(envPath || '').split(path.delimiter).filter(Boolean)];
		return [...new Set(entries)].join(path.delimiter);
	}

	function environment() {
		const providerEnv = deps.providerEnv();
		return { ...providerEnv, PATH: pathWithCliBins(providerEnv.PATH) };
	}

	function isRunning() {
		return process !== null && process.exitCode === null;
	}

	function scheduleRespawn(reason: string) {
		if (deps.context.shouldSuppressSidecarRespawn() || respawnTimer) return;
		if (respawnAttempts >= RESPAWN_MAX_ATTEMPTS) {
			console.error(`CometMind respawn giving up after ${respawnAttempts} attempts (last: ${reason})`);
			return;
		}
		const delay = Math.min(RESPAWN_BASE_MS * 2 ** respawnAttempts, RESPAWN_MAX_MS);
		respawnAttempts += 1;
		console.warn(`CometMind died unexpectedly (${reason}); respawning in ${delay}ms (attempt ${respawnAttempts}/${RESPAWN_MAX_ATTEMPTS})`);
		respawnTimer = setTimeout(async () => {
			respawnTimer = null;
			if (deps.context.shouldSuppressSidecarRespawn() || isRunning()) return;
			start();
			if (await waitForHealth()) respawnAttempts = 0;
		}, delay);
	}

	function start() {
		if (isRunning()) return;
		if (process && process.exitCode !== null) process = null;
		const binary = deps.resolveBinary();
		if (!fs.existsSync(binary)) {
			console.error(`CometMind binary not found: ${binary}`);
			return;
		}
		const logStream = createRotatingLogWriter(deps.getLogPath());
		const child = spawn(binary, ['serve', '--port', String(COMETMIND_PORT), '--watch-parent'], {
			stdio: ['ignore', 'pipe', 'pipe'],
			env: environment()
		});
		process = child;
		child.stdout?.on('data', (data: Buffer) => logStream.write(data));
		child.stderr?.on('data', (data: Buffer) => logStream.write(data));
		child.on('exit', (code) => {
			console.log(`CometMind exited with code ${code}`);
			logStream.end();
			if (process === child) {
				process = null;
				scheduleRespawn(`exit code ${code}`);
			}
		});
		child.on('error', (error) => {
			console.error('CometMind spawn error:', error);
			logStream.end();
			if (process === child) {
				process = null;
				scheduleRespawn(`spawn error: ${error.message}`);
			}
		});
	}

	function runCommand(args: string[]) {
		const binary = deps.resolveBinary();
		if (!fs.existsSync(binary)) return Promise.reject(new Error(`CometMind binary not found: ${binary}`));
		return new Promise<{ stdout: string; stderr: string }>((resolve, reject) => {
			const child = spawn(binary, args, { stdio: ['ignore', 'pipe', 'pipe'], env: environment() });
			let stdout = '';
			let stderr = '';
			child.stdout?.on('data', (data: Buffer) => (stdout += String(data)));
			child.stderr?.on('data', (data: Buffer) => (stderr += String(data)));
			child.on('error', reject);
			child.on('exit', (code) => {
				if (code === 0) return resolve({ stdout, stderr });
				reject(new Error(stderr.trim() || stdout.trim() || `CometMind ${args.join(' ')} exited with code ${code}`));
			});
		});
	}

	function stopProcess(proc: ChildProcess | null, clear: () => void) {
		if (!proc) return Promise.resolve();
		clear();
		return new Promise<void>((resolve) => {
			let settled = false;
			let forceTimer: NodeJS.Timeout | null = null;
			const finish = () => {
				if (settled) return;
				settled = true;
				if (forceTimer) clearTimeout(forceTimer);
				resolve();
			};
			if (proc.exitCode !== null) return finish();
			proc.once('exit', finish);
			forceTimer = setTimeout(() => {
				try {
					proc.kill('SIGKILL');
				} catch {
					// Ignore a process that has already exited.
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

	function stop() {
		if (respawnTimer) clearTimeout(respawnTimer);
		respawnTimer = null;
		respawnAttempts = 0;
		const current = process;
		process = null;
		return stopProcess(current, () => undefined);
	}

	function startGateway(settings: unknown) {
		if (gatewayProcess) return;
		const discord = (settings as { cometmind?: { gateway?: { discord?: { botToken?: unknown } } } })?.cometmind?.gateway?.discord ?? {};
		if (!String(discord.botToken ?? '').trim() && !globalThis.process.env.DISCORD_BOT_TOKEN) {
			console.error('Discord gateway: bot token is not configured');
			return;
		}
		const binary = deps.resolveBinary();
		if (!fs.existsSync(binary)) {
			console.error(`CometMind binary not found: ${binary}`);
			return;
		}
		const logStream = createRotatingLogWriter(deps.getGatewayLogPath());
		const child = spawn(binary, ['gateway', 'run', '--platform', 'discord'], { stdio: ['ignore', 'pipe', 'pipe'], env: environment() });
		gatewayProcess = child;
		child.stdout?.on('data', (data: Buffer) => logStream.write(data));
		child.stderr?.on('data', (data: Buffer) => logStream.write(data));
		child.on('exit', (code) => {
			console.log(`Discord gateway exited with code ${code}`);
			logStream.end();
			gatewayProcess = null;
		});
		child.on('error', (error) => {
			console.error('Discord gateway spawn error:', error);
			logStream.end();
			gatewayProcess = null;
		});
	}

	function stopGateway() {
		const current = gatewayProcess;
		gatewayProcess = null;
		return stopProcess(current, () => undefined);
	}

	async function waitForHealth() {
		for (let i = 0; i < MAX_RETRIES; i += 1) {
			try {
				if ((await fetch(HEALTH_URL, { signal: AbortSignal.timeout(1000) })).ok) return true;
			} catch {
				// Keep polling while the sidecar warms up.
			}
			await new Promise((resolve) => setTimeout(resolve, POLL_MS));
		}
		return false;
	}

	return {
		installCliShim() {
			if (globalThis.process.platform === 'win32') return;
			const binary = deps.resolveBinary();
			if (!fs.existsSync(binary)) return;
			for (const dir of cliBinDirs()) {
				try {
					fs.mkdirSync(dir, { recursive: true });
					const shim = path.join(dir, 'cometmind');
					try {
						const stat = fs.lstatSync(shim);
						if (!stat.isSymbolicLink()) continue;
						if (fs.readlinkSync(shim) === binary) continue;
						fs.unlinkSync(shim);
					} catch (error) {
						if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
					}
					fs.symlinkSync(binary, shim);
				} catch (error) {
					console.warn(`Unable to install CometMind CLI shim in ${dir}:`, error);
				}
			}
		},
		start,
		stop,
		async reload() {
			if (!isRunning()) {
				start();
				return { action: 'restart', healthy: await waitForHealth() };
			}
			try {
				await runCommand(['settings', 'reload']);
				const healthy = await waitForHealth();
				if (!healthy) throw new Error('CometMind did not report healthy after reload');
				return { action: 'reload', healthy: true };
			} catch (error) {
				console.warn('CometMind reload failed, falling back to restart:', error);
				await stop();
				start();
				return { action: 'restart-fallback', healthy: await waitForHealth(), error: error instanceof Error ? error.message : String(error) };
			}
		},
		waitForHealth,
		async syncDiscordGateway(settings) {
			const enabled = Boolean((settings as { cometmind?: { gateway?: { discord?: { enabled?: unknown } } } })?.cometmind?.gateway?.discord?.enabled);
			await stopGateway();
			if (enabled) startGateway(settings);
		},
		isGatewayRunning: () => Boolean(gatewayProcess),
		terminateForExit() {
			for (const child of [gatewayProcess, process]) {
				try {
					child?.kill('SIGTERM');
				} catch {
					// Ignore a process that already exited.
				}
			}
		}
	};
}
