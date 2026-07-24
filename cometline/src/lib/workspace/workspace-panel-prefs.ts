export type WorkspacePanelTreeSource = 'wiki' | 'workspace' | 'changes';
export type MarkdownFileViewMode = 'preview' | 'source';

const TREE_SOURCE_KEY = 'cometline.workspacePanelTreeSource';
const LEGACY_TREE_SOURCE_KEY = 'cometline.webPanelTreeSource';
const MD_VIEW_MODE_KEY = 'cometline.mdFileViewMode';

export function readWorkspacePanelTreeSource(): WorkspacePanelTreeSource {
	try {
		const value =
			localStorage.getItem(TREE_SOURCE_KEY) ?? localStorage.getItem(LEGACY_TREE_SOURCE_KEY);
		if (value === 'wiki' || value === 'workspace' || value === 'changes') return value;
	} catch {
		// ignore
	}
	return 'wiki';
}

export function writeWorkspacePanelTreeSource(source: WorkspacePanelTreeSource): void {
	try {
		localStorage.setItem(TREE_SOURCE_KEY, source);
		localStorage.removeItem(LEGACY_TREE_SOURCE_KEY);
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
