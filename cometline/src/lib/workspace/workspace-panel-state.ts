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
	| { mode: 'url'; url: string; tabId?: string; title?: string };

export type UrlTabMeta = {
	url: string;
	title: string;
};

export type WorkspacePanelState = {
	visible: boolean;
	surface: WorkspacePanelSurface;
	terminalVisible: boolean;
	contentSurface: ContentSurface;
	content: Partial<Record<SurfaceContentKey, SurfaceContent>>;
	/**
	 * Ordered tab ids per tabbed surface.
	 * wiki/workspace ids are file paths; web-search ids are stable slot ids.
	 * Active file id is content[surface].filePath; active url id is content.tabId.
	 */
	tabs: Partial<Record<TabSurfaceKey, string[]>>;
	/** Location + remembered chip title, keyed by web-search tab id. */
	urlTabMeta: Record<string, UrlTabMeta>;
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
		tabs: {},
		urlTabMeta: {}
	};
}

let urlTabSeq = 0;

export function createUrlTabId(): string {
	urlTabSeq += 1;
	return `url-tab-${urlTabSeq}`;
}

export function isBlankTabUrl(url: string | null | undefined): boolean {
	return Boolean(url && (url === 'about:blank' || url.startsWith('about:blank#')));
}

export function isDisplayTabTitle(title: string, url: string): boolean {
	const trimmed = title.trim();
	if (!trimmed) return false;
	if (trimmed === url) return false;
	if (/^https?:\/\//i.test(trimmed)) return false;
	return true;
}

export function urlTabMetaFor(state: WorkspacePanelState, id: string): UrlTabMeta {
	return (state.urlTabMeta ?? {})[id] ?? { url: id, title: '' };
}

export function urlTabChipLabel(meta: UrlTabMeta, liveTitle = ''): string {
	if (!meta.url || isBlankTabUrl(meta.url)) return 'New Tab';
	const title = isDisplayTabTitle(liveTitle, meta.url) ? liveTitle.trim() : meta.title.trim();
	if (isDisplayTabTitle(title, meta.url)) return title;
	return meta.url.replace(/^https?:\/\//, '').split('/')[0] || meta.url;
}

export function isTabSurface(surface: ContentSurface): surface is TabSurfaceKey {
	return surface === 'wiki' || surface === 'workspace' || surface === 'web-search';
}

export function activeTabId(
	content: SurfaceContent | undefined
): string | null {
	if (content?.mode === 'file') return content.filePath;
	if (content?.mode === 'url') return content.tabId ?? content.url;
	return null;
}

export function tabsFor(state: WorkspacePanelState, surface: TabSurfaceKey): string[] {
	const listed = state.tabs[surface];
	if (listed && listed.length > 0) return listed;
	const id = activeTabId(state.content[surface]);
	return id ? [id] : [];
}

function contentForTab(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	id: string,
	reveal?: FileRevealRange | null
): SurfaceContent {
	if (surface === 'web-search') {
		const meta = urlTabMetaFor(state, id);
		return { mode: 'url', url: meta.url, tabId: id, title: meta.title };
	}
	return {
		mode: 'file',
		filePath: id,
		...(reveal ? { startLine: reveal.startLine, endLine: reveal.endLine } : {})
	};
}

function pruneUrlTabMeta(
	state: WorkspacePanelState,
	remainingIds: string[]
): Record<string, UrlTabMeta> {
	if (remainingIds.length === 0) return {};
	const next: Record<string, UrlTabMeta> = {};
	for (const id of remainingIds) {
		const meta = state.urlTabMeta?.[id];
		if (meta) next[id] = meta;
	}
	return next;
}

function withTabs(
	state: WorkspacePanelState,
	surface: TabSurfaceKey,
	tabs: string[]
): WorkspacePanelState {
	const nextTabs = { ...state.tabs };
	if (tabs.length === 0) delete nextTabs[surface];
	else nextTabs[surface] = tabs;
	const next: WorkspacePanelState = { ...state, tabs: nextTabs };
	if (surface === 'web-search') {
		next.urlTabMeta = pruneUrlTabMeta(next, tabs);
	}
	return next;
}

function findUrlTabIdByUrl(state: WorkspacePanelState, url: string): string | undefined {
	for (const id of tabsFor(state, 'web-search')) {
		if (urlTabMetaFor(state, id).url === url) return id;
	}
	return undefined;
}

export function resolveUrlTabId(state: WorkspacePanelState, idOrUrl: string): string | null {
	const tabs = tabsFor(state, 'web-search');
	if (tabs.includes(idOrUrl)) return idOrUrl;
	return findUrlTabIdByUrl(state, idOrUrl) ?? null;
}

function rememberUrlTab(
	state: WorkspacePanelState,
	id: string,
	url: string,
	title: string
): WorkspacePanelState {
	return {
		...state,
		urlTabMeta: {
			...state.urlTabMeta,
			[id]: { url, title }
		}
	};
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
			content: { ...state.content, [surface]: contentForTab(state, surface, id, reveal) }
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
			content: { ...state.content, [surface]: contentForTab(state, surface, id) }
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
			content: { ...state.content, [surface]: contentForTab(state, surface, id) }
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
				content: { ...state.content, [surface]: contentForTab(state, surface, nextId) }
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
	nextContent: SurfaceContent | null
): boolean {
	if (nextContent?.mode === 'file') return false;
	const active = state.content[state.contentSurface];
	if (active?.mode !== 'file') return false;
	return true;
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
	const existing = findUrlTabIdByUrl(state, url);
	if (existing) return activatePanelTab(state, 'web-search', existing);
	const id = createUrlTabId();
	return openPanelTab(rememberUrlTab(state, id, url, ''), 'web-search', id);
}

export function navigateWorkspacePanelUrl(
	state: WorkspacePanelState,
	url: string
): WorkspacePanelState {
	const active = activeTabId(state.content['web-search']);
	if (!active) return openWorkspacePanelUrl(state, url);
	return {
		...state,
		visible: true,
		surface: 'web',
		contentSurface: 'web-search',
		urlTabMeta: {
			...state.urlTabMeta,
			[active]: { url, title: '' }
		},
		content: {
			...state.content,
			'web-search': { mode: 'url', url, tabId: active, title: '' }
		}
	};
}

/** Guest navigation updates its owning tab without activating it or reopening the panel. */
export function syncUrlTab(
	state: WorkspacePanelState,
	id: string,
	url: string,
	title = ''
): WorkspacePanelState {
	if (!urlTabsFor(state).includes(id)) return state;
	const content = state.content['web-search'];
	const active = activeTabId(content) === id;
	const prev = urlTabMetaFor(state, id);
	const nextTitle = isDisplayTabTitle(title, url)
		? title.trim()
		: prev.title;
	if (prev.url === url && prev.title === nextTitle) {
		return state;
	}
	return {
		...state,
		urlTabMeta: {
			...state.urlTabMeta,
			[id]: { url, title: nextTitle }
		},
		content: active
			? { ...state.content, 'web-search': { mode: 'url', url, tabId: id, title: nextTitle } }
			: state.content
	};
}

export function activateWorkspacePanelUrlTab(
	state: WorkspacePanelState,
	idOrUrl: string
): WorkspacePanelState {
	const id = resolveUrlTabId(state, idOrUrl);
	if (!id) return state;
	return activatePanelTab(state, 'web-search', id);
}

export function closeWorkspacePanelUrlTab(
	state: WorkspacePanelState,
	idOrUrl: string
): WorkspacePanelState {
	const id = resolveUrlTabId(state, idOrUrl);
	if (!id) return state;
	return closePanelTab(state, 'web-search', id);
}
