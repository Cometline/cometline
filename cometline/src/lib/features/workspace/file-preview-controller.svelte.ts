import { tick, untrack } from 'svelte';
import {
	listWikiFileBacklinks,
	readWikiFileContent,
	readWorkspaceFileContent,
	writeWikiFileContent,
	writeWorkspaceFileContent
} from '$lib/client/cometmind';
import { shellStore } from '$lib/stores/shell.svelte';
import {
	isMarkdownPath,
	isPdfPath,
	languageFromExtension,
	languageFromPath,
	shouldSkipTextPreviewReload
} from '$lib/features/workspace/file-preview';
import {
	buildFileSnippetContext,
	sourceLineRangeFromDomRange,
	type SelectionLineRange
} from '$lib/features/workspace/selection-snippet';
import {
	firstSelectionClientRect,
	selectionPopupPosition
} from '$lib/features/workspace/selection-popup';
import type { FileRevealRange } from '$lib/features/workspace/workspace-panel-state';
import {
	readMarkdownFileViewMode,
	writeMarkdownFileViewMode,
	type MarkdownFileViewMode
} from '$lib/features/workspace/workspace-panel-prefs';
import { refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';
import { workspaceFileChangeVersion } from '$lib/features/workspace/workspace-change.svelte';
import { createFileDiff } from '$lib/features/workspace/file-diff';
import {
	highlightGitDiffLines,
	type HighlightedDiffLine
} from '$lib/features/workspace/git-diff-highlight';
import { parseGitDiffLines } from '$lib/features/workspace/git-diff-lines';
import { isWikiReadOnlyPath, isWikiUiPath, toWikiRelative } from '$lib/wiki/paths';

export type FilePreviewEditorState = {
	dirty: boolean;
	saving: boolean;
	saveError: string | null;
	save: () => Promise<void>;
	revert: () => void;
};

export function createFilePreviewController(deps: {
	getWorkspacePath: () => string;
	getFilePath: () => string;
	getRevealRange: () => FileRevealRange | null;
	onEditorState?: (state: FilePreviewEditorState | null) => void;
}) {
	const s = $state({
		loading: true,
		error: null as string | null,
		imageDataUrl: '',
		savedContent: '',
		draftContent: '',
		language: null as string | null,
		previewKind: null as 'text' | 'image' | 'pdf' | null,
		saving: false,
		saveError: null as string | null,
		viewMode: readMarkdownFileViewMode() as MarkdownFileViewMode,
		wikiFiles: [] as string[],
		backlinks: [] as string[],
		backlinksLoading: false,
		externalChangePending: false,
		externalComparisonLines: null as HighlightedDiffLine[] | null,
		externalComparisonError: null as string | null,
		externalComparisonOpen: false,
		pdfReloadVersion: 0,
		markdownScrollEl: null as HTMLDivElement | null,
		fileEditor: null as {
			getSelectionRange: () => {
				text: string;
				startLine: number;
				endLine: number;
				clientRect: DOMRect;
			} | null;
		} | null,
		selectionPopup: null as {
			top: number;
			left: number;
			text: string;
			lineRange: SelectionLineRange | null;
		} | null
	});
	let loadVersion = 0;
	let lastObservedFileChangeVersion = 0;

	const readOnly = $derived(
		isWikiUiPath(deps.getFilePath()) && isWikiReadOnlyPath(deps.getFilePath())
	);
	const dirty = $derived(
		s.previewKind === 'text' && s.draftContent !== s.savedContent && !readOnly
	);
	const isMarkdown = $derived(isMarkdownPath(deps.getFilePath()));
	const isPdf = $derived(isPdfPath(deps.getFilePath()));
	const showMarkdownToggle = $derived(s.previewKind === 'text' && isMarkdown);
	const effectiveViewMode = $derived(
		showMarkdownToggle ? s.viewMode : ('source' satisfies MarkdownFileViewMode)
	);
	const isWikiFile = $derived(isWikiUiPath(deps.getFilePath()));
	const showBacklinks = $derived(
		isWikiFile && s.previewKind === 'text' && !s.loading && !s.error
	);

	function setViewMode(mode: MarkdownFileViewMode) {
		s.viewMode = mode;
		writeMarkdownFileViewMode(mode);
		s.selectionPopup = null;
	}

	function clearSelectionPopup() {
		s.selectionPopup = null;
	}

	function placeSelectionPopup(
		text: string,
		clientRect: DOMRect,
		lineRange: SelectionLineRange | null
	) {
		if (!text.trim()) {
			clearSelectionPopup();
			return;
		}
		s.selectionPopup = {
			...selectionPopupPosition(clientRect, window.innerWidth),
			text,
			lineRange
		};
	}

	function onPreviewMouseUp(event: MouseEvent) {
		const root = event.currentTarget;
		if (!(root instanceof HTMLElement)) return;
		const sel = window.getSelection();
		if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
			clearSelectionPopup();
			return;
		}
		if (!root.contains(sel.anchorNode) || !root.contains(sel.focusNode)) {
			clearSelectionPopup();
			return;
		}
		const range = sel.getRangeAt(0);
		const text = sel.toString();
		placeSelectionPopup(
			text,
			firstSelectionClientRect(range),
			sourceLineRangeFromDomRange(range, root)
		);
	}

	function onSourceMouseUp() {
		const selected = s.fileEditor?.getSelectionRange() ?? null;
		if (!selected) {
			clearSelectionPopup();
			return;
		}
		placeSelectionPopup(selected.text, selected.clientRect, {
			startLine: selected.startLine,
			endLine: selected.endLine
		});
	}

	function addSelectionToChat() {
		if (!s.selectionPopup) return;
		const ctx = buildFileSnippetContext({
			filePath: deps.getFilePath(),
			selectedText: s.selectionPopup.text,
			sourceText: s.draftContent,
			lineRange: s.selectionPopup.lineRange
		});
		if (ctx) {
			shellStore.addWebContextForActive(ctx);
			shellStore.requestComposerFocus();
		}
		clearSelectionPopup();
		window.getSelection()?.removeAllRanges();
	}

	function revert() {
		if (s.previewKind !== 'text') return;
		s.draftContent = s.savedContent;
		s.saveError = null;
	}

	function keepEditingAfterExternalChange() {
		s.externalChangePending = false;
		s.externalComparisonLines = null;
		s.externalComparisonError = null;
		s.externalComparisonOpen = false;
	}

	function reloadAfterExternalChange() {
		keepEditingAfterExternalChange();
		void loadPreview();
	}

	async function compareExternalChange() {
		if (s.externalComparisonOpen) {
			s.externalComparisonLines = null;
			s.externalComparisonError = null;
			s.externalComparisonOpen = false;
			return;
		}
		const currentWorkspacePath = deps.getWorkspacePath();
		const currentFilePath = deps.getFilePath();
		const currentDraftContent = s.draftContent;
		s.externalComparisonError = null;
		clearSelectionPopup();
		try {
			const result = isWikiUiPath(currentFilePath)
				? await readWikiFileContent(toWikiRelative(currentFilePath))
				: await readWorkspaceFileContent(currentWorkspacePath, currentFilePath);
			if (
				deps.getWorkspacePath() !== currentWorkspacePath ||
				deps.getFilePath() !== currentFilePath
			)
				return;
			if (result.kind !== 'text') {
				s.externalComparisonError = 'The external version is not text.';
				return;
			}
			const diff = createFileDiff(currentDraftContent, result.content);
			const lines = await highlightGitDiffLines(
				parseGitDiffLines(diff),
				languageFromPath(currentFilePath) ?? languageFromExtension(result.extension)
			);
			if (
				deps.getWorkspacePath() !== currentWorkspacePath ||
				deps.getFilePath() !== currentFilePath ||
				s.draftContent !== currentDraftContent
			)
				return;
			s.externalComparisonLines = lines;
			s.externalComparisonOpen = true;
		} catch (err) {
			if (
				deps.getWorkspacePath() !== currentWorkspacePath ||
				deps.getFilePath() !== currentFilePath
			)
				return;
			s.externalComparisonError =
				err instanceof Error ? err.message : 'Failed to load the external file version';
		}
	}

	async function save() {
		if (s.previewKind !== 'text' || s.saving || !dirty || readOnly) return;

		const nextContent = s.draftContent;
		const currentWorkspacePath = deps.getWorkspacePath();
		const currentFilePath = deps.getFilePath();

		s.saving = true;
		s.saveError = null;
		try {
			if (isWikiUiPath(currentFilePath)) {
				await writeWikiFileContent(toWikiRelative(currentFilePath), nextContent);
			} else {
				await writeWorkspaceFileContent(currentWorkspacePath, currentFilePath, nextContent);
			}
			if (
				deps.getWorkspacePath() !== currentWorkspacePath ||
				deps.getFilePath() !== currentFilePath
			)
				return;
			s.savedContent = nextContent;
			s.draftContent = nextContent;
			if (isWikiUiPath(currentFilePath)) {
				void refreshWikiFileIndex(true);
				void loadBacklinks(currentFilePath);
			}
		} catch (err) {
			if (
				deps.getWorkspacePath() !== currentWorkspacePath ||
				deps.getFilePath() !== currentFilePath
			)
				return;
			s.saveError = err instanceof Error ? err.message : 'Failed to save file';
		} finally {
			if (
				deps.getWorkspacePath() === currentWorkspacePath &&
				deps.getFilePath() === currentFilePath
			) {
				s.saving = false;
			}
		}
	}

	async function loadBacklinks(path: string) {
		if (!isWikiUiPath(path)) {
			s.backlinks = [];
			return;
		}
		s.backlinksLoading = true;
		try {
			s.backlinks = await listWikiFileBacklinks(toWikiRelative(path));
		} catch {
			s.backlinks = [];
		} finally {
			s.backlinksLoading = false;
		}
	}

	async function loadPreview(opts?: { keepView?: boolean }) {
		const version = ++loadVersion;
		const keepView = Boolean(opts?.keepView) && s.previewKind !== null && !s.error;
		if (!keepView) {
			s.loading = true;
			s.error = null;
			s.imageDataUrl = '';
			s.savedContent = '';
			s.draftContent = '';
			s.language = null;
			s.previewKind = null;
			s.saving = false;
			s.saveError = null;
			s.backlinks = [];
			s.selectionPopup = null;
		}

		try {
			if (isPdf) {
				s.previewKind = 'pdf';
				return;
			}
			const wikiIndexPromise = refreshWikiFileIndex(true);
			const result = isWikiUiPath(deps.getFilePath())
				? await readWikiFileContent(toWikiRelative(deps.getFilePath()))
				: await readWorkspaceFileContent(deps.getWorkspacePath(), deps.getFilePath());
			if (version !== loadVersion) return;

			s.wikiFiles = await wikiIndexPromise;
			if (version !== loadVersion) return;

			if (result.kind === 'image') {
				if (keepView && s.previewKind === 'image' && s.imageDataUrl === result.data_url)
					return;
				s.previewKind = 'image';
				s.imageDataUrl = result.data_url;
				return;
			}

			if (
				shouldSkipTextPreviewReload(keepView, s.previewKind, s.savedContent, result.content)
			) {
				return;
			}

			const markdownScrollTop = s.markdownScrollEl?.scrollTop ?? null;
			s.savedContent = result.content;
			s.draftContent = result.content;
			s.language =
				languageFromPath(deps.getFilePath()) ?? languageFromExtension(result.extension);
			s.previewKind = 'text';
			void loadBacklinks(deps.getFilePath());
			if (markdownScrollTop !== null) {
				await tick();
				if (version !== loadVersion) return;
				if (s.markdownScrollEl) s.markdownScrollEl.scrollTop = markdownScrollTop;
			}
		} catch (err) {
			if (version !== loadVersion) return;
			s.error = err instanceof Error ? err.message : 'Failed to load file';
		} finally {
			if (version === loadVersion) s.loading = false;
		}
	}

	$effect(() => {
		// Track both inputs so the editor reloads when either changes.
		void [deps.getWorkspacePath(), deps.getFilePath()];
		lastObservedFileChangeVersion = untrack(() =>
			workspaceFileChangeVersion(deps.getWorkspacePath(), deps.getFilePath())
		);
		s.externalChangePending = false;
		s.externalComparisonLines = null;
		s.externalComparisonError = null;
		s.externalComparisonOpen = false;
		void loadPreview();
	});

	$effect(() => {
		const changeVersion = workspaceFileChangeVersion(
			deps.getWorkspacePath(),
			deps.getFilePath()
		);
		if (changeVersion === lastObservedFileChangeVersion) return;
		lastObservedFileChangeVersion = changeVersion;
		if (isPdf) {
			s.pdfReloadVersion += 1;
			return;
		}
		if (dirty) {
			s.externalChangePending = true;
			s.externalComparisonLines = null;
			s.externalComparisonError = null;
			s.externalComparisonOpen = false;
			return;
		}
		void loadPreview({ keepView: true });
	});

	$effect(() => {
		// Jumping to a line range requires the source editor, not markdown preview.
		if (deps.getRevealRange() && isMarkdown && s.viewMode === 'preview') {
			s.viewMode = 'source';
			writeMarkdownFileViewMode('source');
		}
	});

	$effect(() => {
		deps.onEditorState?.(
			s.previewKind === 'text' && !s.loading && !s.error && !readOnly
				? {
						dirty,
						saving: s.saving,
						saveError: s.saveError,
						save,
						revert
					}
				: null
		);
	});

	$effect(() => {
		return () => {
			deps.onEditorState?.(null);
		};
	});

	return {
		s,
		get readOnly() {
			return readOnly;
		},
		get dirty() {
			return dirty;
		},
		get isMarkdown() {
			return isMarkdown;
		},
		get isPdf() {
			return isPdf;
		},
		get showMarkdownToggle() {
			return showMarkdownToggle;
		},
		get effectiveViewMode() {
			return effectiveViewMode;
		},
		get isWikiFile() {
			return isWikiFile;
		},
		get showBacklinks() {
			return showBacklinks;
		},
		setViewMode,
		clearSelectionPopup,
		placeSelectionPopup,
		onPreviewMouseUp,
		onSourceMouseUp,
		addSelectionToChat,
		revert,
		keepEditingAfterExternalChange,
		reloadAfterExternalChange,
		compareExternalChange,
		save,
		loadBacklinks,
		loadPreview
	};
}

export type FilePreviewController = ReturnType<typeof createFilePreviewController>;
