import { getActiveSessionId } from '#lib/active-session.js';

export type FocusedPane = 'chat' | 'web' | 'terminal';

export type ComposerFocusRequest = {
	id: number;
	sessionId: string | null;
};

/** Focused pane plus monotonically increasing focus-request ids consumed by components. */
export function createShellFocusStore() {
	let focusedPane = $state<FocusedPane>('chat');
	let addressBarFocusRequestId = $state(0);
	let fileTreeFilterFocusRequestId = $state(0);
	let gitChangesOpenRequestId = $state(0);
	let terminalFocusRequestId = $state(0);
	/** Last focus target while the web slot is open: filter vs web address. */
	let lastWorkspacePanelFocusTarget = $state<'filter' | 'address'>('filter');
	let composerFocusRequest = $state<ComposerFocusRequest>({ id: 0, sessionId: null });
	let sessionFindRequestId = $state(0);

	return {
		get focusedPane() {
			return focusedPane;
		},
		get addressBarFocusRequestId() {
			return addressBarFocusRequestId;
		},
		get fileTreeFilterFocusRequestId() {
			return fileTreeFilterFocusRequestId;
		},
		get gitChangesOpenRequestId() {
			return gitChangesOpenRequestId;
		},
		/** Last ⌘O target while browsing: filter vs web address. */
		get lastWorkspacePanelFocusTarget(): 'filter' | 'address' {
			return lastWorkspacePanelFocusTarget;
		},
		get terminalFocusRequestId() {
			return terminalFocusRequestId;
		},
		get composerFocusRequest() {
			return composerFocusRequest;
		},
		get sessionFindRequestId() {
			return sessionFindRequestId;
		},
		setFocusedPane(pane: FocusedPane) {
			focusedPane = pane;
		},
		setLastWorkspacePanelFocusTarget(target: 'filter' | 'address') {
			lastWorkspacePanelFocusTarget = target;
		},
		bumpFileTreeFilterFocus() {
			lastWorkspacePanelFocusTarget = 'filter';
			fileTreeFilterFocusRequestId += 1;
		},
		bumpAddressBarFocus() {
			lastWorkspacePanelFocusTarget = 'address';
			addressBarFocusRequestId += 1;
		},
		bumpGitChangesOpen() {
			gitChangesOpenRequestId += 1;
		},
		bumpTerminalFocus() {
			terminalFocusRequestId += 1;
		},
		requestComposerFocus(sessionId = getActiveSessionId()) {
			focusedPane = 'chat';
			composerFocusRequest = { id: composerFocusRequest.id + 1, sessionId };
		},
		requestSessionFind() {
			focusedPane = 'chat';
			sessionFindRequestId += 1;
		}
	};
}

export type ShellFocusStore = ReturnType<typeof createShellFocusStore>;
