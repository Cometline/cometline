import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { beforeAll, describe, expect, it, vi, type Mock } from 'vitest';

import { registerIpcHandlers, type IpcHandlers } from '../domains/ipc.js';
import { EVENT_CHANNELS, INVOKE_CHANNELS, SEND_CHANNELS } from './ipc-channels.js';

const electron = vi.hoisted(() => ({
	exposeInMainWorld: vi.fn(),
	rendererInvoke: vi.fn(() => Promise.resolve()),
	rendererSend: vi.fn(),
	rendererOn: vi.fn(),
	mainHandle: vi.fn(),
	mainOn: vi.fn()
}));

vi.mock('electron', () => ({
	contextBridge: { exposeInMainWorld: electron.exposeInMainWorld },
	ipcRenderer: {
		invoke: electron.rendererInvoke,
		send: electron.rendererSend,
		on: electron.rendererOn,
		removeListener: vi.fn()
	},
	ipcMain: { handle: electron.mainHandle, on: electron.mainOn }
}));

const electronSrc = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function channelsCalled(mock: Mock) {
	return new Set(mock.mock.calls.map(([channel]) => channel as string));
}

function mainProcessSources() {
	const files: string[] = [];
	const walk = (directory: string) => {
		for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
			const full = path.join(directory, entry.name);
			if (entry.isDirectory()) walk(full);
			else if (entry.name.endsWith('.ts') && !entry.name.endsWith('.test.ts'))
				files.push(full);
		}
	};
	walk(electronSrc);
	return files
		.filter((file) => !file.startsWith(path.join(electronSrc, 'shared')))
		.filter((file) => file !== path.join(electronSrc, 'preload.ts'))
		.map((file) => ({ file, source: fs.readFileSync(file, 'utf8') }));
}

let exposedMethods: string[] = [];

beforeAll(async () => {
	await import('../preload.js');
	const api = electron.exposeInMainWorld.mock.calls[0]?.[1] as Record<
		string,
		(...args: unknown[]) => unknown
	>;
	exposedMethods = Object.keys(api);
	for (const method of Object.values(api)) method(() => {});
	registerIpcHandlers(new Proxy({}, { get: () => () => undefined }) as IpcHandlers);
});

describe('IPC channel registry', () => {
	it('names each channel after the preload method that uses it', () => {
		const registryMethods = [
			...Object.keys(INVOKE_CHANNELS),
			...Object.keys(SEND_CHANNELS),
			...Object.keys(EVENT_CHANNELS)
		];
		expect(new Set(registryMethods).size).toBe(registryMethods.length);
		expect(new Set(exposedMethods)).toEqual(new Set(registryMethods));
	});

	it.each([
		['invoke', INVOKE_CHANNELS],
		['send', SEND_CHANNELS],
		['event', EVENT_CHANNELS]
	])('has no duplicate %s channel names', (_kind, channels) => {
		const values = Object.values(channels);
		expect(new Set(values).size).toBe(values.length);
	});

	it('registers a main handler for every channel the preload invokes, and no others', () => {
		const invoked = channelsCalled(electron.rendererInvoke);
		expect(invoked).toEqual(new Set(Object.values(INVOKE_CHANNELS)));
		expect(channelsCalled(electron.mainHandle)).toEqual(invoked);
	});

	it('registers a main listener for every channel the preload sends, and no others', () => {
		const sent = channelsCalled(electron.rendererSend);
		expect(sent).toEqual(new Set(Object.values(SEND_CHANNELS)));
		expect(channelsCalled(electron.mainOn)).toEqual(sent);
	});

	it('subscribes the preload to every event channel', () => {
		expect(channelsCalled(electron.rendererOn)).toEqual(new Set(Object.values(EVENT_CHANNELS)));
	});

	it('has a main-process sender for every event channel', () => {
		const sources = mainProcessSources()
			.map(({ source }) => source)
			.join('\n');
		const unsent = Object.keys(EVENT_CHANNELS).filter(
			(method) => !sources.includes(`EVENT_CHANNELS.${method}`)
		);
		expect(unsent).toEqual([]);
	});

	it('keeps raw channel strings out of main-process code', () => {
		const literal = /['"`](?:cometline|cometmind|jobs):[a-z-]+['"`]/;
		const offenders = mainProcessSources()
			.filter(({ source }) => literal.test(source))
			.map(({ file }) => path.relative(electronSrc, file));
		expect(offenders).toEqual([]);
	});
});
