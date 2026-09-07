export type WorkspaceOwnedPane = 'web' | 'terminal';
export type WorkspaceFocusTarget = 'address' | 'filter' | 'panel';

export function isWorkspaceOwnedPane(pane: string): pane is WorkspaceOwnedPane {
	return pane === 'web' || pane === 'terminal';
}

/** Composer context chips live in `<main>` but are not a chat-pane claim. */
export function isComposerContextChipTarget(node: EventTarget | null): boolean {
	if (!(node instanceof Element)) return false;
	return Boolean(node.closest('.message-context-chips'));
}

/**
 * Main-window pointer should take the chat ring only for real chat content,
 * not for context-chip actions such as opening a referenced workspace file.
 */
export function shouldClaimChatPaneFromMainPointer(target: EventTarget | null): boolean {
	return !isComposerContextChipTarget(target);
}

/**
 * Park focus from current panel content, not from a stale "request" flag.
 * Electron often steals focus after the address request was already marked done.
 */
export function resolveWorkspaceFocusTarget(opts: {
	focusedPane: string;
	showContentSearch: boolean;
	addressEditing: boolean;
	isBlankTab: boolean;
	showWebview: boolean;
	showBrowseFilter: boolean;
	hasFilePreview: boolean;
}): WorkspaceFocusTarget {
	if (opts.focusedPane === 'terminal') return 'panel';
	if (opts.showContentSearch && (opts.isBlankTab || opts.addressEditing)) return 'address';
	if (opts.showBrowseFilter && !opts.hasFilePreview) return 'filter';
	return 'panel';
}
