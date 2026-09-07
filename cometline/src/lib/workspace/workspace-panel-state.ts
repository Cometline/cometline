export type WorkspacePanelSurface = 'web' | 'terminal';
export type SurfaceContentKey = 'wiki' | 'workspace' | 'changes' | 'web-search';
export type ContentSurface = SurfaceContentKey;
export type FileSurfaceKey = Extract<SurfaceContentKey, 'wiki' | 'workspace'>;
/** Surfaces that own an ordered tab strip (active id lives in content). */
export type TabSurfaceKey = FileSurfaceKey | 'web-search';

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
	/**
	 * Ordered tab ids per tabbed surface.
	 * wiki/workspace ids are file paths; web-search ids are URLs.
	 * Active id is content[surface].filePath or content[surface].url.
	 */
	tabs: Partial<Record<TabSurfaceKey, string[]>>;
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
		tabs: {}
	};
}

export function isTabSurface(surface: ContentSurface): surface is TabSurfaceKey {
	return surface === 'wiki' || surface === 'workspace' || surface === 'web-search';
}

export function activeTabId(
	content: SurfaceContent | undefined
): string | null {
	if (content?.mode === 'file') return content.filePath;
	if (content?.mode === 'url') return content.url;
	return null;
}

export function tabsFor(state: WorkspacePanelState, surface: TabSurfaceKey): string[] {
	const listed = state.tabs[surface];
	if (listed && listed.length > 0) return listed;
	const id = activeTabId(state.content[surface]);
	return id ? [id] : [];
}

function contentForTab(
	surface: TabSurfaceKey,
	id: string,
	reveal?: FileRevealRange | null
): SurfaceContent {
	if (surface === 'web-search') return { mode: 'url', url: id };
	return {
		mode: 'file',
		filePath: id,
		...(reveal ? { startLine: reveal.startLine, endLine: reveal.endLine } : {})
	};
}

function withTabs(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	tabs: string[]
): WorkspacePanelState {
	const nextTabs = { ...state.tabs };
	if (tabs.length === 0) delete nextTabs[surface];
	else nextTabs[surface] = tabs;
	return { ...state, tabs: nextTabs };
}

/** Add or activate a tab on a tabbed surface. */
export function openPanelTab(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	id: string,
	reveal?: FileRevealRange | null
): WorkspacePanelState {
	const tabs = tabsFor(state, surface);
	const nextTabs = tabs.includes(id) ? tabs : [...tabs, id];
	return withTabs(
		{
			...state,
			visible: true,
			surface: 'web',
			contentSurface: surface,
			content: { ...state.content, [surface]: contentForTab(surface, id, reveal) }
		},
		surface,
		nextTabs
	);
}

export function activatePanelTab(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	id: string
): WorkspacePanelState {
	const tabs = tabsFor(state, surface);
	if (!tabs.includes(id)) return state;
	return withTabs(
		{
			...state,
			visible: true,
			surface: 'web',
			contentSurface: surface,
			content: { ...state.content, [surface]: contentForTab(surface, id) }
		},
		surface,
		tabs
	);
}

/** Replace the active tab id in place (address-bar navigate), or open if none. */
export function replaceActivePanelTab(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	id: string
): WorkspacePanelState {
	const active = activeTabId(state.content[surface]);
	if (!active) return openPanelTab(state, surface, id);
	const tabs = tabsFor(state, surface);
	const nextTabs = tabs.map((tab) => (tab === active ? id : tab));
	const deduped: string[] = [];
	for (const tab of nextTabs) {
		if (!deduped.includes(tab)) deduped.push(tab);
	}
	const finalTabs = deduped.includes(id) ? deduped : [...deduped, id];
	return withTabs(
		{
			...state,
			visible: true,
			surface: 'web',
			contentSurface: surface,
			content: { ...state.content, [surface]: contentForTab(surface, id) }
		},
		surface,
		finalTabs
	);
}

/** Close one tab; if it is active, fall through Cmd+W step. */
export function closePanelTab(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	id: string
): WorkspacePanelState {
	const tabs = tabsFor(state, surface);
	if (!tabs.includes(id)) return state;
	const active = activeTabId(state.content[surface]);
	if (active === id) {
		if (state.contentSurface !== surface) {
			state = { ...state, contentSurface: surface, surface: 'web', visible: true };
		}
		return closeWorkspacePanel(state);
	}
	return withTabs(state, surface, tabs.filter((tab) => tab !== id));
}

function closeActiveTabStep(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	activeId: string
): WorkspacePanelState {
	const tabs = tabsFor(state, surface);
	const idx = tabs.indexOf(activeId);
	if (tabs.length > 1) {
		const nextTabs = tabs.filter((tab) => tab !== activeId);
		const nextId = nextTabs[Math.min(Math.max(idx, 0), nextTabs.length - 1)];
		return withTabs(
			{
				...state,
				content: { ...state.content, [surface]: contentForTab(surface, nextId) }
			},
			surface,
			nextTabs
		);
	}
	const content = { ...state.content };
	delete content[surface];
	return withTabs({ ...state, content }, surface, []);
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
 * Tabbed surfaces close the active tab first; last tab clears content.
 */
export function closeWorkspacePanel(state: WorkspacePanelState): WorkspacePanelState {
	if (state.surface === 'terminal') {
		return { ...state, terminalVisible: false };
	}

	const surface = state.contentSurface;
	const activeContent = state.content[surface];
	const activeId = activeTabId(activeContent);
	if (isTabSurface(surface) && activeId) {
		return closeActiveTabStep(state, surface, activeId);
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

// --- Compatibility wrappers (call sites / readability) ---

export function fileTabsFor(state: WorkspacePanelState, surface: FileSurfaceKey): string[] {
	return tabsFor(state, surface);
}

export function urlTabsFor(state: WorkspacePanelState): string[] {
	return tabsFor(state, 'web-search');
}

export function openWorkspacePanelFile(
	state: WorkspacePanelState,
	surface: FileSurfaceKey,
	filePath: string,
	reveal?: FileRevealRange | null
): WorkspacePanelState {
	return openPanelTab(state, surface, filePath, reveal);
}

export function activateWorkspacePanelFileTab(
	state: WorkspacePanelState,
	surface: FileSurfaceKey,
	filePath: string
): WorkspacePanelState {
	return activatePanelTab(state, surface, filePath);
}

export function closeWorkspacePanelFileTab(
	state: WorkspacePanelState,
	surface: FileSurfaceKey,
	filePath: string
): WorkspacePanelState {
	return closePanelTab(state, surface, filePath);
}

export function openWorkspacePanelUrl(
	state: WorkspacePanelState,
	url: string
): WorkspacePanelState {
	return openPanelTab(state, 'web-search', url);
}

export function navigateWorkspacePanelUrl(
	state: WorkspacePanelState,
	url: string
): WorkspacePanelState {
	return replaceActivePanelTab(state, 'web-search', url);
}

export function activateWorkspacePanelUrlTab(
	state: WorkspacePanelState,
	url: string
): WorkspacePanelState {
	return activatePanelTab(state, 'web-search', url);
}

export function closeWorkspacePanelUrlTab(
	state: WorkspacePanelState,
	url: string
): WorkspacePanelState {
	return closePanelTab(state, 'web-search', url);
}
