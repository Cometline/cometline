import { tick } from 'svelte';
import {
	resetCustomCaret,
	scheduleCustomCaretMeasure,
	type CustomCaretState
} from '$lib/dom/custom-caret';
import { openLink } from '$lib/open-link';
import { shellStore } from '$lib/stores/shell.svelte';
import { openWorkspaceFilePreview } from '$lib/features/workspace/open-file-preview';
import { isSelectionAtEditorEdge } from '$lib/features/composer/composer-caret';
import { makeDirChip, makeFileChip } from '$lib/features/composer/rich-composer-chips';
import {
	decorateEditor,
	serialize,
	type DecorateOptions
} from '$lib/features/composer/rich-composer-decorate';
import {
	findActiveMention,
	replaceRangeWithNodes,
	setCaretPosition
} from '$lib/features/composer/rich-composer-selection';

export function createRichComposerInputController(deps: {
	getEditor: () => HTMLDivElement | null;
	setValue: (value: string) => void;
	getSkillNames: () => string[];
	getMentionsEnabled: () => boolean;
	onkeydown: (e: KeyboardEvent) => void;
	onfiles: (files: File[]) => void;
	onmentionquery: (payload: { query: string; active: boolean }) => void;
}) {
	let focused = $state(false);
	let caretReady = $state(false);
	// Guard so our own DOM writes don't recursively re-trigger input handling.
	let syncing = false;
	// IME composition guard — Enter during candidate selection must not trigger send.
	let composing = false;
	// Mention state used by the parent composer to show/hide the file picker.
	let lastMentionActive = $state(false);
	let lastMentionQuery = $state('');

	function readValue() {
		const editor = deps.getEditor();
		if (!editor) return;
		let next = serialize(editor);
		// Empty contenteditable often has a lone <br>; don't treat that as content.
		if (next === '\n' && !editor.textContent) next = '';
		deps.setValue(next);
	}

	function scheduleCaretMeasure() {
		scheduleCustomCaretMeasure(deps.getEditor());
	}

	function resetCaretTrail() {
		resetCustomCaret(deps.getEditor());
	}

	let queuedCaretState: CustomCaretState | null = null;
	let caretStateUpdateQueued = false;

	function onCaretStateChange(state: CustomCaretState) {
		queuedCaretState = state;
		if (caretStateUpdateQueued) return;
		caretStateUpdateQueued = true;
		queueMicrotask(() => {
			caretStateUpdateQueued = false;
			if (!queuedCaretState) return;
			focused = queuedCaretState.focused;
			caretReady = queuedCaretState.ready;
			queuedCaretState = null;
		});
	}

	function activeMention() {
		return findActiveMention(deps.getEditor(), deps.getMentionsEnabled());
	}

	function updateMentionState() {
		if (!deps.getEditor()) return;
		const mention = activeMention();
		const active = mention !== null;
		const query = mention?.query ?? '';
		if (active !== lastMentionActive || query !== lastMentionQuery) {
			lastMentionActive = active;
			lastMentionQuery = query;
			deps.onmentionquery({ query, active });
		}
	}

	function decorate(opts?: DecorateOptions) {
		return decorateEditor(deps.getEditor(), deps.getSkillNames(), opts);
	}

	function syncDecorations(opts?: DecorateOptions) {
		syncing = true;
		decorate(opts);
		syncing = false;
		readValue();
		scheduleCaretMeasure();
	}

	function onInput() {
		if (syncing || !deps.getEditor()) return;
		syncDecorations();
		updateMentionState();
	}

	function onPaste(e: ClipboardEvent) {
		const files = Array.from(e.clipboardData?.files ?? []).filter((file) =>
			file.type.startsWith('image/')
		);
		if (files.length > 0) {
			e.preventDefault();
			deps.onfiles(files);
			return;
		}

		// Force plain-text paste so we don't inherit foreign HTML, then linkify
		// immediately so a pasted URL becomes a chip without needing an extra
		// keystroke.
		const text = e.clipboardData?.getData('text/plain');
		if (text == null) return;
		e.preventDefault();
		document.execCommand('insertText', false, text);
		if (!deps.getEditor()) return;
		syncDecorations({ allowCaretEnd: true });
	}

	function onCompositionStart() {
		composing = true;
	}

	function onCompositionEnd() {
		// Defer reset: some browsers fire the confirming keydown after compositionend.
		setTimeout(() => {
			composing = false;
			scheduleCaretMeasure();
		}, 0);
	}

	function onKeydownInternal(e: KeyboardEvent) {
		if ((composing || e.isComposing) && !e.ctrlKey && !e.metaKey && !e.altKey) return;
		deps.onkeydown(e);
	}

	function onEditorClick(e: MouseEvent) {
		const target = e.target;
		if (!(target instanceof Element)) return;
		const chip = target.closest('.rce-chip');
		if (!(chip instanceof HTMLElement)) return;
		// A plain click on a file chip opens it in the side-panel editor.
		// Directory chips open the WorkspacePanel file browser.
		if (chip.dataset.filePath) {
			e.preventDefault();
			const path = chip.dataset.filePath;
			if (chip.classList.contains('rce-dir-chip') || path.endsWith('/')) {
				shellStore.openWorkspacePanelBrowse();
				return;
			}
			openWorkspaceFilePreview(path);
			return;
		}
		// A plain click on a URL chip opens its link.
		if (chip.dataset.url) {
			e.preventDefault();
			openLink(chip.dataset.url);
		}
	}

	function focus(options?: { position?: 'start' | 'end' }) {
		const editor = deps.getEditor();
		editor?.focus({ preventScroll: true });
		if (!editor) return;
		const atEnd =
			options?.position === 'end' ||
			(options?.position !== 'start' && Boolean(editor.textContent?.length));
		setCaretPosition(editor, atEnd);
		scheduleCaretMeasure();
	}

	async function focusAsync(options?: { position?: 'start' | 'end' }) {
		await tick();
		focus(options);
	}

	function insertText(text: string) {
		if (!deps.getEditor()) return;
		focus();
		document.execCommand('insertText', false, text);
		readValue();
		scheduleCaretMeasure();
	}

	function insertFileMention(path: string) {
		const editor = deps.getEditor();
		if (!editor) return;
		const mention = activeMention();
		const chip = path.endsWith('/') ? makeDirChip(path) : makeFileChip(path);
		const space = document.createTextNode('\u00a0');
		syncing = true;
		if (mention) {
			replaceRangeWithNodes(mention.range, [chip, space]);
		} else {
			editor.focus({ preventScroll: true });
			const sel = window.getSelection();
			const range = sel && sel.rangeCount > 0 ? sel.getRangeAt(0) : null;
			if (range && editor.contains(range.commonAncestorContainer)) {
				replaceRangeWithNodes(range, [chip, space]);
			} else {
				editor.append(chip, space);
				const endRange = document.createRange();
				endRange.selectNodeContents(editor);
				endRange.collapse(false);
				sel?.removeAllRanges();
				sel?.addRange(endRange);
			}
		}
		decorate();
		syncing = false;
		readValue();
		scheduleCaretMeasure();
		updateMentionState();
	}

	function getFilePaths(): string[] {
		const editor = deps.getEditor();
		if (!editor) return [];
		const chips = editor.querySelectorAll('.rce-file-chip, .rce-dir-chip');
		const paths: string[] = [];
		for (const chip of chips) {
			const path = (chip as HTMLElement).dataset.filePath;
			if (path) paths.push(path);
		}
		return paths;
	}

	function setText(text: string) {
		const editor = deps.getEditor();
		if (!editor) {
			deps.setValue(text);
			return;
		}
		editor.textContent = text;
		deps.setValue(text);
		focus({ position: 'end' });
		syncDecorations({ allowCaretEnd: true });
	}

	/** Clears the editor (used after send). */
	function clear() {
		const editor = deps.getEditor();
		if (editor) editor.innerHTML = '';
		deps.setValue('');
		resetCaretTrail();
		// Editor often keeps focus after send, but innerHTML = '' drops the selection.
		// Re-seat the caret and remeasure so the custom caret layer becomes visible.
		if (focused && editor) {
			editor.focus({ preventScroll: true });
			setCaretPosition(editor, false);
			scheduleCaretMeasure();
		}
	}

	function isCaretAtStart(): boolean {
		return isSelectionAtEditorEdge(deps.getEditor(), 'start');
	}

	function isCaretAtEnd(): boolean {
		return isSelectionAtEditorEdge(deps.getEditor(), 'end');
	}

	return {
		get focused() {
			return focused;
		},
		get caretReady() {
			return caretReady;
		},
		onCaretStateChange,
		updateMentionState,
		syncDecorations,
		onInput,
		onPaste,
		onCompositionStart,
		onCompositionEnd,
		onKeydownInternal,
		onEditorClick,
		focus,
		focusAsync,
		insertText,
		insertFileMention,
		getFilePaths,
		setText,
		clear,
		isCaretAtStart,
		isCaretAtEnd
	};
}
