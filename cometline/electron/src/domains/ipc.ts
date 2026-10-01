import { ipcMain, type IpcMainEvent, type IpcMainInvokeEvent } from 'electron';

import {
	INVOKE_CHANNELS,
	SEND_CHANNELS,
	type InvokeMethod,
	type SendMethod
} from '../shared/ipc-channels.js';

type Listener = (event: IpcMainEvent, ...args: unknown[]) => void;
type Invoker = (event: IpcMainInvokeEvent, ...args: unknown[]) => unknown;

export type IpcHandlers = { [K in SendMethod]: Listener } & { [K in InvokeMethod]: Invoker };

/** Registers a main-process handler for every renderer → main channel in the registry. */
export function registerIpcHandlers(handlers: IpcHandlers) {
	for (const method of Object.keys(SEND_CHANNELS) as SendMethod[]) {
		ipcMain.on(SEND_CHANNELS[method], handlers[method]);
	}
	for (const method of Object.keys(INVOKE_CHANNELS) as InvokeMethod[]) {
		ipcMain.handle(INVOKE_CHANNELS[method], handlers[method]);
	}
}
