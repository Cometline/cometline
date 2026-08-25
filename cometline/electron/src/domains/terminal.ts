import { app, BrowserWindow, type IpcMainInvokeEvent } from 'electron';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import pty, { type IPty } from 'node-pty';

import {
	clearAllTerminalEnv,
	integrationScriptPath,
	prepareEnvDir,
	removeTerminalEnvDir,
	SESSION_ID_RE,
	shellIntegrationRoot,
	shellKind,
	spawnArgs,
	writeBashRc,
	writeZshDotDir,
	zshDotDir
} from './terminal-env.js';

const BUFFER_MAX_CHARS = 2_000_000;
const MIN_COLS = 2;
const MAX_COLS = 500;
const MIN_ROWS = 1;
const MAX_ROWS = 500;

interface TerminalEntry {
	sessionId: string;
	status: 'running' | 'exited';
	exitCode: number | null;
	generation: number;
	shell: string;
	output: string;
	process: IPty | null;
}

export interface TerminalSnapshot {
	sessionId: string;
	status: 'running' | 'exited';
	exitCode: number | null;
	generation: number;
	shell: string;
	output: string;
}

export interface TerminalCreateInput {
	sessionId: string;
	workspacePath: string;
	cols?: number;
	rows?: number;
}

export function createTerminalManager(getMainWindow: () => BrowserWindow | null) {
	const sessions = new Map<string, TerminalEntry>();
	clearAllTerminalEnv();

	const dimensions = (input: Pick<TerminalCreateInput, 'cols' | 'rows'> = {}) => {
		const cols = Number.isInteger(input.cols) ? (input.cols ?? 80) : 80;
		const rows = Number.isInteger(input.rows) ? (input.rows ?? 24) : 24;
		return {
			cols: Math.min(MAX_COLS, Math.max(MIN_COLS, cols)),
			rows: Math.min(MAX_ROWS, Math.max(MIN_ROWS, rows))
		};
	};

	const snapshot = (entry: TerminalEntry): TerminalSnapshot => ({
		sessionId: entry.sessionId,
		status: entry.status,
		exitCode: entry.exitCode,
		generation: entry.generation,
		shell: entry.shell,
		output: entry.output
	});

	const send = (channel: string, payload: unknown) => {
		const window = getMainWindow();
		if (window && !window.isDestroyed()) window.webContents.send(channel, payload);
	};

	const isMainWindowSender = (event: IpcMainInvokeEvent) => {
		const window = getMainWindow();
		return Boolean(window && !window.isDestroyed() && event.sender === window.webContents);
	};

	const requireInput = (event: IpcMainInvokeEvent, sessionId: unknown) => {
		if (!isMainWindowSender(event))
			throw new Error('Terminal access is only available in the main window');
		if (typeof sessionId !== 'string' || !SESSION_ID_RE.test(sessionId)) {
			throw new Error('Invalid terminal session id');
		}
	};

	const shell = () => {
		let shellPath = process.env.SHELL;
		try {
			const userShell = os.userInfo().shell;
			if (!shellPath && userShell) shellPath = userShell;
		} catch {
			// Fall through to macOS's default interactive shell.
		}
		if (
			typeof shellPath === 'string' &&
			path.isAbsolute(shellPath) &&
			fs.existsSync(shellPath)
		) {
			return shellPath;
		}
		return '/bin/zsh';
	};

	const integrationRoot = () =>
		shellIntegrationRoot(app.isPackaged, process.resourcesPath, app.getAppPath());

	const spawnEnv = (
		kind: ReturnType<typeof shellKind>,
		envDir: string,
		zshWrapperReady: boolean
	) => {
		const env: NodeJS.ProcessEnv = {
			...process.env,
			TERM: 'xterm-256color',
			TERM_PROGRAM: 'Cometline',
			COMETLINE_ENV_DIR: envDir
		};
		if (kind !== 'zsh' || !zshWrapperReady) return env;
		const zdot = zshDotDir(envDir);
		env.COMETLINE_ZDOTDIR = zdot;
		env.COMETLINE_USER_ZDOTDIR = process.env.ZDOTDIR || os.homedir();
		env.ZDOTDIR = zdot;
		return env;
	};

	const appendOutput = (entry: TerminalEntry, data: string) => {
		entry.output += data;
		if (entry.output.length > BUFFER_MAX_CHARS)
			entry.output = entry.output.slice(-BUFFER_MAX_CHARS);
	};

	const prepareEnvCapture = (sessionId: string, shellPath: string) => {
		const envDir = prepareEnvDir(sessionId);
		const kind = shellKind(shellPath);
		const script = integrationScriptPath(integrationRoot(), shellPath);
		const scriptReady = Boolean(script && fs.existsSync(script));
		let bashRc: string | null = null;
		let zshWrapperReady = false;
		if (kind === 'zsh' && scriptReady) {
			writeZshDotDir(envDir, script);
			zshWrapperReady = true;
		} else if (kind === 'bash' && scriptReady) {
			bashRc = writeBashRc(envDir, script);
		}
		return { envDir, kind, bashRc, zshWrapperReady };
	};

	const create = (sessionId: string, workspacePath: string, input: TerminalCreateInput) => {
		const existing = sessions.get(sessionId);
		if (existing?.status === 'running') return snapshot(existing);
		if (typeof workspacePath !== 'string' || !path.isAbsolute(workspacePath)) {
			throw new Error('Terminal workspace must be an absolute path');
		}
		let stat: fs.Stats;
		try {
			stat = fs.statSync(workspacePath);
		} catch {
			throw new Error('Terminal workspace does not exist');
		}
		if (!stat.isDirectory()) throw new Error('Terminal workspace must be a directory');

		const size = dimensions(input);
		const shellPath = shell();
		const capture = prepareEnvCapture(sessionId, shellPath);
		const processHandle = pty.spawn(shellPath, spawnArgs(capture.kind, capture.bashRc), {
			name: 'xterm-256color',
			cols: size.cols,
			rows: size.rows,
			cwd: workspacePath,
			env: spawnEnv(capture.kind, capture.envDir, capture.zshWrapperReady)
		});
		const entry: TerminalEntry = {
			sessionId,
			status: 'running',
			exitCode: null,
			generation: (existing?.generation ?? 0) + 1,
			shell: shellPath,
			output: '',
			process: processHandle
		};
		sessions.set(sessionId, entry);
		processHandle.onData((data) => {
			appendOutput(entry, data);
			send('cometline:terminal-data', { sessionId, data });
		});
		processHandle.onExit(({ exitCode }) => {
			entry.status = 'exited';
			entry.exitCode = exitCode;
			entry.process = null;
			removeTerminalEnvDir(sessionId);
			send('cometline:terminal-exit', snapshot(entry));
			sessions.delete(sessionId);
		});
		return snapshot(entry);
	};

	const terminate = (sessionId: string, remove = false) => {
		const entry = sessions.get(sessionId);
		if (!entry) return false;
		if (entry.status === 'running' && entry.process) {
			try {
				entry.process.kill();
			} catch (error) {
				console.warn(`Failed to terminate terminal for session ${sessionId}:`, error);
			}
		}
		removeTerminalEnvDir(sessionId);
		if (remove) sessions.delete(sessionId);
		return true;
	};

	return {
		create,
		dimensions,
		isMainWindowSender,
		list: () => [...sessions.values()].map(snapshot),
		requireInput,
		terminate,
		terminateAll: () => {
			for (const sessionId of sessions.keys()) terminate(sessionId, true);
			clearAllTerminalEnv();
		},
		write: (sessionId: string, data: string) => {
			const entry = sessions.get(sessionId);
			if (!entry || entry.status !== 'running' || !entry.process) return false;
			if (typeof data !== 'string' || data.length > 64 * 1024)
				throw new Error('Invalid terminal input');
			entry.process.write(data);
			return true;
		},
		resize: (sessionId: string, input: TerminalCreateInput) => {
			const entry = sessions.get(sessionId);
			if (!entry || entry.status !== 'running' || !entry.process) return false;
			const size = dimensions(input);
			entry.process.resize(size.cols, size.rows);
			return true;
		}
	};
}
