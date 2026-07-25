export type WorkspacePanelSurface = 'web' | 'terminal';
export type SurfaceContentKey = 'wiki' | 'workspace' | 'changes' | 'web-search';
export type ContentSurface = SurfaceContentKey;

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
	return {
		...state,
		visible: true,
		surface: 'web',
		contentSurface: surface,
		content: { ...state.content, [surface]: fileContent }
	};
}

/** Drop one-shot line reveal after the editor has scrolled to it. */
export function clearFileReveal(
	state: WorkspacePanelState,
	surface: Extract<SurfaceContentKey, 'wiki' | 'workspace'>
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
