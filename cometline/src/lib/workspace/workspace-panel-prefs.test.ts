import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
	readMarkdownFileViewMode,
	readWorkspacePanelTreeSource,
	writeMarkdownFileViewMode,
	writeWorkspacePanelTreeSource
} from './workspace-panel-prefs';

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

describe('workspace-panel-prefs', () => {
	beforeEach(() => {
		installLocalStorageMock();
	});

	afterEach(() => {
		localStorage.clear();
	});

	it('defaults tree source to wiki', () => {
		expect(readWorkspacePanelTreeSource()).toBe('wiki');
	});

	it('persists tree source', () => {
		writeWorkspacePanelTreeSource('workspace');
		expect(readWorkspacePanelTreeSource()).toBe('workspace');
	});

	it('reads legacy tree source key', () => {
		localStorage.setItem('cometline.webPanelTreeSource', 'changes');
		expect(readWorkspacePanelTreeSource()).toBe('changes');
	});

	it('defaults markdown view mode to preview', () => {
		expect(readMarkdownFileViewMode()).toBe('preview');
	});

	it('persists markdown view mode', () => {
		writeMarkdownFileViewMode('source');
		expect(readMarkdownFileViewMode()).toBe('source');
	});
});
