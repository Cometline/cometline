import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
	readMarkdownFileViewMode,
	readWebPanelTreeSource,
	writeMarkdownFileViewMode,
	writeWebPanelTreeSource
} from './web-panel-prefs';

function installLocalStorageMock() {
	const store = new Map<string, string>();
	const mock = {
		getItem: (key: string) => store.get(key) ?? null,
		setItem: (key: string, value: string) => {
			store.set(key, value);
		},
		removeItem: (key: string) => {
			store.delete(key);
		},
		clear: () => {
			store.clear();
		}
	};
	Object.defineProperty(globalThis, 'localStorage', { value: mock, configurable: true });
}

describe('web-panel-prefs', () => {
	beforeEach(() => {
		installLocalStorageMock();
	});

	afterEach(() => {
		localStorage.clear();
	});

	it('defaults tree source to wiki', () => {
		expect(readWebPanelTreeSource()).toBe('wiki');
	});

	it('persists tree source', () => {
		writeWebPanelTreeSource('workspace');
		expect(readWebPanelTreeSource()).toBe('workspace');
	});

	it('defaults markdown view mode to preview', () => {
		expect(readMarkdownFileViewMode()).toBe('preview');
	});

	it('persists markdown view mode', () => {
		writeMarkdownFileViewMode('source');
		expect(readMarkdownFileViewMode()).toBe('source');
	});
});
