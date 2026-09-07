import { SvelteMap } from 'svelte/reactivity';
import type WorkspaceWebSurface from '$lib/components/WorkspaceWebSurface.svelte';

export type WebTabStatus = Pick<
	ReturnType<typeof WorkspaceWebSurface>['pageState'],
	'showLoading' | 'loadError' | 'audible' | 'muted' | 'ready'
>;

export type WebTabActivity = {
	sessionId: string;
	tabId: string;
	surface: ReturnType<typeof WorkspaceWebSurface>;
};

// References to live guests only. Binding teardown removes entries; nothing is persisted.
export const webTabActivity = new SvelteMap<string, WebTabActivity>();
