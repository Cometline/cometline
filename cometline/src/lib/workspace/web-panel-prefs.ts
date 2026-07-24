export type WebPanelTreeSource = 'wiki' | 'workspace' | 'changes';
export type MarkdownFileViewMode = 'preview' | 'source';

const TREE_SOURCE_KEY = 'cometline.webPanelTreeSource';
const MD_VIEW_MODE_KEY = 'cometline.mdFileViewMode';

export function readWebPanelTreeSource(): WebPanelTreeSource {
	try {
		const value = localStorage.getItem(TREE_SOURCE_KEY);
		if (value === 'wiki' || value === 'workspace' || value === 'changes') return value;
	} catch {
		// ignore
	}
	return 'wiki';
}

export function writeWebPanelTreeSource(source: WebPanelTreeSource): void {
	try {
		localStorage.setItem(TREE_SOURCE_KEY, source);
	} catch {
		// ignore
	}
}

export function readMarkdownFileViewMode(): MarkdownFileViewMode {
	try {
		const value = localStorage.getItem(MD_VIEW_MODE_KEY);
		if (value === 'preview' || value === 'source') return value;
	} catch {
		// ignore
	}
	return 'preview';
}

export function writeMarkdownFileViewMode(mode: MarkdownFileViewMode): void {
	try {
		localStorage.setItem(MD_VIEW_MODE_KEY, mode);
	} catch {
		// ignore
	}
}
