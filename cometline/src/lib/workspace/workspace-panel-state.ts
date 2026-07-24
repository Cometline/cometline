export type WorkspacePanelSurface = 'web' | 'terminal';
export type SurfaceContentKey = 'wiki' | 'workspace' | 'changes' | 'web-search';
export type ContentSurface = SurfaceContentKey;

export type SurfaceContent =
	| { mode: 'file'; filePath: string }
	| { mode: 'git-diff'; filePath: string }
	| { mode: 'url'; url: string };

export type WorkspacePanelState = {
	visible: boolean;
	surface: WorkspacePanelSurface;
	terminalVisible: boolean;
	contentSurface: ContentSurface;
	content: Partial<Record<SurfaceContentKey, SurfaceContent>>;
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
		content: {}
	};
}

export function openWorkspacePanelFile(
	state: WorkspacePanelState,
	surface: Extract<SurfaceContentKey, 'wiki' | 'workspace'>,
	filePath: string
): WorkspacePanelState {
	return {
		...state,
		visible: true,
		surface: 'web',
		contentSurface: surface,
		content: { ...state.content, [surface]: { mode: 'file', filePath } }
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
 */
export function closeWorkspacePanel(state: WorkspacePanelState): WorkspacePanelState {
	if (state.surface === 'terminal') {
		return { ...state, terminalVisible: false };
	}

	const activeContent = state.content[state.contentSurface];
	if (activeContent) {
		const content = { ...state.content };
		delete content[state.contentSurface];
		return { ...state, content };
	}

	const nextSurface = nextSurfaceWithContent(state);
	if (nextSurface) {
		return { ...state, visible: true, surface: 'web', contentSurface: nextSurface };
	}

	return { ...state, visible: false };
}

export function replacesActiveFile(
	state: WorkspacePanelState,
	nextSurface: ContentSurface,
	nextContent: SurfaceContent | null
): boolean {
	const active = state.content[state.contentSurface];
	if (active?.mode !== 'file') return false;
	return (
		nextSurface !== state.contentSurface ||
		nextContent?.mode !== 'file' ||
		nextContent.filePath !== active.filePath
	);
}
