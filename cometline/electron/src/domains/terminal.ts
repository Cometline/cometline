import { BrowserWindow, type IpcMainInvokeEvent } from 'electron';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import pty, { type IPty } from 'node-pty';

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
		if (typeof sessionId !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(sessionId)) {
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

	const appendOutput = (entry: TerminalEntry, data: string) => {
		entry.output += data;
		if (entry.output.length > BUFFER_MAX_CHARS)
			entry.output = entry.output.slice(-BUFFER_MAX_CHARS);
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
		const processHandle = pty.spawn(shellPath, ['-l'], {
			name: 'xterm-256color',
			cols: size.cols,
			rows: size.rows,
			cwd: workspacePath,
			env: { ...process.env, TERM: 'xterm-256color', TERM_PROGRAM: 'Cometline' }
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
