export type WorkspacePanelSurface = 'web' | 'terminal';
export type SurfaceContentKey = 'wiki' | 'workspace' | 'changes' | 'web-search';
export type ContentSurface = SurfaceContentKey;
export type FileSurfaceKey = Extract<SurfaceContentKey, 'wiki' | 'workspace'>;

export type FileRevealRange = {
	startLine: number;
	endLine: number;
};

export type SurfaceContent =
	| { mode: 'file'; filePath: string; startLine?: number; endLine?: number }
	| { mode: 'git-diff'; filePath: string }
	| { mode: 'url'; url: string };

export type WorkspacePanelState = {
	visible: boolean;
	surface: WorkspacePanelSurface;
	terminalVisible: boolean;
	contentSurface: ContentSurface;
	content: Partial<Record<SurfaceContentKey, SurfaceContent>>;
	/** Open file tabs per wiki/workspace surface; active tab is content[surface].filePath. */
	fileTabs: Partial<Record<FileSurfaceKey, string[]>>;
};

export const SURFACE_CLOSE_ORDER: SurfaceContentKey[] = [
	'wiki',
	'workspace',
	'web-search',
	'changes'
];

export function createWorkspacePanelState(
	contentSurface: ContentSurface
): WorkspacePanelState {
	return {
		visible: false,
		surface: 'web',
		terminalVisible: false,
		contentSurface,
		content: {},
		fileTabs: {}
	};
}

export function fileTabsFor(
	state: WorkspacePanelState,
	surface: FileSurfaceKey
): string[] {
	const listed = state.fileTabs[surface];
	if (listed && listed.length > 0) return listed;
	const content = state.content[surface];
	if (content?.mode === 'file') return [content.filePath];
	return [];
}

export function openWorkspacePanelFile(
	state: WorkspacePanelState,
	surface: FileSurfaceKey,
	filePath: string,
	reveal?: FileRevealRange | null
): WorkspacePanelState {
	const fileContent: Extract<SurfaceContent, { mode: 'file' }> = {
		mode: 'file',
		filePath,
		...(reveal
			? { startLine: reveal.startLine, endLine: reveal.endLine }
			: {})
	};
	const tabs = fileTabsFor(state, surface);
	const nextTabs = tabs.includes(filePath) ? tabs : [...tabs, filePath];
	return {
		...state,
		visible: true,
		surface: 'web',
		contentSurface: surface,
		content: { ...state.content, [surface]: fileContent },
		fileTabs: { ...state.fileTabs, [surface]: nextTabs }
	};
}

export function activateWorkspacePanelFileTab(
	state: WorkspacePanelState,
	surface: FileSurfaceKey,
	filePath: string
): WorkspacePanelState {
	const tabs = fileTabsFor(state, surface);
	if (!tabs.includes(filePath)) return state;
	return {
		...state,
		visible: true,
		surface: 'web',
		contentSurface: surface,
		content: {
			...state.content,
			[surface]: { mode: 'file', filePath }
		},
		fileTabs: { ...state.fileTabs, [surface]: tabs }
	};
}

/** Drop one-shot line reveal after the editor has scrolled to it. */
export function clearFileReveal(
	state: WorkspacePanelState,
	surface: FileSurfaceKey
): WorkspacePanelState {
	const content = state.content[surface];
	if (content?.mode !== 'file') return state;
	if (content.startLine == null && content.endLine == null) return state;
	return {
		...state,
		content: {
			...state.content,
			[surface]: { mode: 'file', filePath: content.filePath }
		}
	};
}

export function nextSurfaceWithContent(
	state: WorkspacePanelState
): SurfaceContentKey | null {
	const start = SURFACE_CLOSE_ORDER.indexOf(state.contentSurface);
	if (start < 0) return null;

	for (let i = 1; i < SURFACE_CLOSE_ORDER.length; i++) {
		const candidate = SURFACE_CLOSE_ORDER[(start + i) % SURFACE_CLOSE_ORDER.length];
		if (state.content[candidate]) return candidate;
	}
	return null;
}

/**
 * Applies one Cmd+W step without touching focus, history, or persistence.
 * The shell store owns those effects; this module owns the panel transition.
 *
 * For file surfaces: close the active tab first; only when the last tab is gone
 * clear surface content (then existing walk-dots / hide cascade applies).
 */
export function closeWorkspacePanel(state: WorkspacePanelState): WorkspacePanelState {
	if (state.surface === 'terminal') {
		return { ...state, terminalVisible: false };
	}

	const surface = state.contentSurface;
	const activeContent = state.content[surface];
	if (
		activeContent?.mode === 'file' &&
		(surface === 'wiki' || surface === 'workspace')
	) {
		const tabs = fileTabsFor(state, surface);
		const activePath = activeContent.filePath;
		const idx = tabs.indexOf(activePath);
		if (tabs.length > 1) {
			const nextTabs = tabs.filter((path) => path !== activePath);
			const nextPath = nextTabs[Math.min(Math.max(idx, 0), nextTabs.length - 1)];
			return {
				...state,
				content: {
					...state.content,
					[surface]: { mode: 'file', filePath: nextPath }
				},
				fileTabs: { ...state.fileTabs, [surface]: nextTabs }
			};
		}
		const content = { ...state.content };
		delete content[surface];
		const fileTabs = { ...state.fileTabs };
		delete fileTabs[surface];
		return { ...state, content, fileTabs };
	}

	if (activeContent) {
		const content = { ...state.content };
		delete content[surface];
		return { ...state, content };
	}

	const nextSurface = nextSurfaceWithContent(state);
	if (nextSurface) {
		return { ...state, visible: true, surface: 'web', contentSurface: nextSurface };
	}

	return { ...state, visible: false };
}

/**
 * True when navigation would destroy the active file buffer.
 * Multi-tab keep-alive: opening/activating another file never destroys a tab.
 */
export function replacesActiveFile(
	state: WorkspacePanelState,
	nextSurface: ContentSurface,
	nextContent: SurfaceContent | null
): boolean {
	if (nextContent?.mode === 'file') return false;
	const active = state.content[state.contentSurface];
	if (active?.mode !== 'file') return false;
	return nextSurface !== state.contentSurface || nextContent?.mode !== 'file';
}
