export type ComposerFocusPane = 'chat' | 'web' | 'terminal';

export function shouldApplyComposerFocus(opts: {
	requestId: number;
	requestSessionId: string | null;
	sessionId: string;
	focusedPane: ComposerFocusPane;
	lastAppliedRequestId: number;
}): boolean {
	if (!opts.requestId) return false;
	if (opts.requestSessionId !== opts.sessionId) return false;
	if (opts.focusedPane !== 'chat') return false;
	if (opts.requestId === opts.lastAppliedRequestId) return false;
	return true;
}
