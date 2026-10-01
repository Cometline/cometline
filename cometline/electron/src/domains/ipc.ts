import { ipcMain, type IpcMainEvent, type IpcMainInvokeEvent } from 'electron';

import {
	INVOKE_CHANNELS,
	SEND_CHANNELS,
	type InvokeMethod,
	type InvokeResponse,
	type SendMethod
} from '../shared/ipc-channels.js';

// Renderer arguments cross a trust boundary, so handlers take `unknown` and
// validate; only their results are held to the ElectronAPI contract.
type Listener = (event: IpcMainEvent, ...args: unknown[]) => void;
type Invoker<K extends InvokeMethod> = (
	event: IpcMainInvokeEvent,
	...args: unknown[]
) => InvokeResponse<K> | Promise<InvokeResponse<K>>;

export type IpcHandlers = { [K in SendMethod]: Listener } & {
	[K in InvokeMethod]: Invoker<K>;
};

/** Registers a main-process handler for every renderer → main channel in the registry. */
export function registerIpcHandlers(handlers: IpcHandlers) {
	for (const method of Object.keys(SEND_CHANNELS) as SendMethod[]) {
		ipcMain.on(SEND_CHANNELS[method], handlers[method]);
	}
	for (const method of Object.keys(INVOKE_CHANNELS) as InvokeMethod[]) {
		ipcMain.handle(INVOKE_CHANNELS[method], handlers[method]);
	}
}
