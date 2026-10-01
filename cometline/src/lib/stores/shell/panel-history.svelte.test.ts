import { describe, expect, it } from 'vitest';
import { createPanelHistoryStore } from './panel-history.svelte';

describe('createPanelHistoryStore', () => {
	it('steps back and forward through recorded entries per surface', () => {
		const history = createPanelHistoryStore();
		history.record('s1', 'workspace', { kind: 'browse', source: 'workspace' });
		history.record('s1', 'workspace', { kind: 'file', path: 'src/a.ts' });

		expect(history.canGoBack('s1', 'workspace')).toBe(true);
		expect(history.canGoBack('s1', 'wiki')).toBe(false);
		expect(history.stepBack('s1', 'workspace')).toEqual({
			kind: 'browse',
			source: 'workspace'
		});
		expect(history.canGoForward('s1', 'workspace')).toBe(true);
		expect(history.stepForward('s1', 'workspace')).toEqual({ kind: 'file', path: 'src/a.ts' });
		expect(history.stepForward('s1', 'workspace')).toBeNull();
	});

	it('does not record while applying an entry', () => {
		const history = createPanelHistoryStore();
		history.whileApplying(() => {
			history.record('s1', 'web-search', { kind: 'url', url: 'https://a.example' });
		});

		expect(history.canGoBack('s1', 'web-search')).toBe(false);
	});

	it('seeds an empty web-search entry under the first url and skips duplicates', () => {
		const history = createPanelHistoryStore();
		history.record('s1', 'web-search', { kind: 'url', url: 'https://a.example' });
		history.record('s1', 'web-search', { kind: 'url', url: 'https://a.example' });

		expect(history.stepBack('s1', 'web-search')).toEqual({ kind: 'url', url: '' });
		expect(history.canGoBack('s1', 'web-search')).toBe(false);
	});

	it('clears all surfaces for a session', () => {
		const history = createPanelHistoryStore();
		history.record('s1', 'wiki', { kind: 'file', path: '@runtime/wiki/index.md' });
		history.clearForSession('s1');

		expect(history.canGoBack('s1', 'wiki')).toBe(false);
	});
});
