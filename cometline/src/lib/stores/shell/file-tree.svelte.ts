import { getActiveSessionId } from '$lib/active-session';
import { dirKeysToExpandForPaths } from '$lib/features/workspace/file-tree';

/** File-tree sources that keep an expansion map (not Changes). */
export type FileTreeExpandSource = 'wiki' | 'workspace';

/** Per-session expansion and filter state for the Wiki/Workspace file trees. */
export function createFileTreeStore() {
	/**
	 * Expanded directory keys for Wiki/Workspace file trees, per session.
	 * Survives open-file → back so the tree does not collapse.
	 */
	let fileTreeExpandedBySession = $state<
		Record<string, Partial<Record<FileTreeExpandSource, Record<string, boolean>>>>
	>({});
	/** Per-session filter text for Wiki/Workspace trees (independent lifecycles). */
	let fileTreeFilterBySession = $state<
		Record<string, Partial<Record<FileTreeExpandSource, string>>>
	>({});

	function fileTreeFilterFor(sessionId: string, source: FileTreeExpandSource): string {
		return fileTreeFilterBySession[sessionId]?.[source] ?? '';
	}

	function setFileTreeFilterForSession(
		sessionId: string,
		source: FileTreeExpandSource,
		value: string
	) {
		fileTreeFilterBySession = {
			...fileTreeFilterBySession,
			[sessionId]: {
				...fileTreeFilterBySession[sessionId],
				[source]: value
			}
		};
	}

	function clearFileTreeFilterForSession(sessionId: string) {
		if (!(sessionId in fileTreeFilterBySession)) return;
		const next = { ...fileTreeFilterBySession };
		delete next[sessionId];
		fileTreeFilterBySession = next;
	}

	function fileTreeExpandedFor(
		sessionId: string,
		source: FileTreeExpandSource
	): Record<string, boolean> {
		return fileTreeExpandedBySession[sessionId]?.[source] ?? {};
	}

	function setFileTreeExpandedForSession(
		sessionId: string,
		source: FileTreeExpandSource,
		expanded: Record<string, boolean>
	) {
		fileTreeExpandedBySession = {
			...fileTreeExpandedBySession,
			[sessionId]: {
				...fileTreeExpandedBySession[sessionId],
				[source]: { ...expanded }
			}
		};
	}

	function clearFileTreeExpandedForSession(sessionId: string) {
		if (!(sessionId in fileTreeExpandedBySession)) return;
		const next = { ...fileTreeExpandedBySession };
		delete next[sessionId];
		fileTreeExpandedBySession = next;
	}

	return {
		/** Merge ancestor dirs for a relative path into the source expansion map. */
		expandToRelativePath(
			sessionId: string,
			source: FileTreeExpandSource,
			relativePath: string
		) {
			const parents = dirKeysToExpandForPaths([relativePath]);
			if (Object.keys(parents).length === 0) return;
			const prev = fileTreeExpandedFor(sessionId, source);
			setFileTreeExpandedForSession(sessionId, source, { ...prev, ...parents });
		},
		clearForSession(sessionId: string) {
			clearFileTreeExpandedForSession(sessionId);
			clearFileTreeFilterForSession(sessionId);
		},
		/** Read expanded dir keys for Wiki or Workspace tree (session-scoped). */
		getFileTreeExpanded(source: FileTreeExpandSource): Record<string, boolean> {
			const key = getActiveSessionId();
			return key ? fileTreeExpandedFor(key, source) : {};
		},
		/** Persist expanded dir keys (user toggles / open-to-path). */
		setFileTreeExpanded(source: FileTreeExpandSource, expanded: Record<string, boolean>) {
			const key = getActiveSessionId();
			if (!key) return;
			setFileTreeExpandedForSession(key, source, expanded);
		},
		getFileTreeFilter(source: FileTreeExpandSource): string {
			const key = getActiveSessionId();
			return key ? fileTreeFilterFor(key, source) : '';
		},
		setFileTreeFilter(source: FileTreeExpandSource, value: string) {
			const key = getActiveSessionId();
			if (!key) return;
			setFileTreeFilterForSession(key, source, value);
		}
	};
}

export type FileTreeStore = ReturnType<typeof createFileTreeStore>;
