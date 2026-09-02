<script lang="ts">
	import { untrack } from 'svelte';
	import { basicSetup, EditorView } from 'codemirror';
	import { Compartment, EditorState, EditorSelection } from '@codemirror/state';
	import { keymap } from '@codemirror/view';
	import type { Extension } from '@codemirror/state';
	import { codemirrorLanguageSupport } from '$lib/workspace/codemirror-language';
	import type { FileRevealRange } from '$lib/workspace/workspace-panel-state';
	import { replaceEditorDocument } from '$lib/workspace/replace-editor-document';

	let {
		value,
		language,
		readOnly = false,
		revealRange = null,
		onChange,
		onSave,
		onRevealApplied
	}: {
		value: string;
		language: string | null;
		readOnly?: boolean;
		revealRange?: FileRevealRange | null;
		onChange?: (value: string) => void;
		onSave?: () => void;
		onRevealApplied?: () => void;
	} = $props();

	let host = $state<HTMLDivElement | null>(null);
	let editorView = $state<EditorView | null>(null);
	let lastAppliedRevealKey = $state<string | null>(null);

	/** Selected text + 1-based line range, or null when empty. */
	export function getSelectionRange(): {
		text: string;
		startLine: number;
		endLine: number;
		clientRect: DOMRect;
	} | null {
		const view = editorView;
		if (!view) return null;
		const { from, to } = view.state.selection.main;
		if (from === to) return null;
		const text = view.state.sliceDoc(from, to);
		if (!text.trim()) return null;
		const startLine = view.state.doc.lineAt(from).number;
		const endLine = view.state.doc.lineAt(to > from ? to - 1 : to).number;
		const coords = view.coordsAtPos(from);
		const clientRect = coords
			? new DOMRect(
					coords.left,
					coords.top,
					coords.right - coords.left,
					coords.bottom - coords.top
				)
			: (host?.getBoundingClientRect() ?? new DOMRect());
		return { text, startLine, endLine, clientRect };
	}
	const languageCompartment = new Compartment();
	const editableCompartment = new Compartment();
	const readOnlyCompartment = new Compartment();

	function editorTheme(): Extension {
		return EditorView.theme({
			'&': {
				height: '100%',
				backgroundColor: '#fff',
				color: 'var(--text-main)',
				fontSize: '12px'
			},
			'.cm-scroller': {
				fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
				lineHeight: '1.5',
				padding: '12px 0 24px'
			},
			'.cm-content': {
				padding: '0 18px'
			},
			'.cm-gutters': {
				backgroundColor: '#fff',
				border: 'none',
				color: 'var(--text-muted)'
			},
			'.cm-lineNumbers .cm-gutterElement': {
				padding: '0 8px 0 12px'
			},
			'.cm-activeLineGutter, .cm-activeLine': {
				backgroundColor: 'rgba(15, 23, 42, 0.035)'
			},
			'.cm-focused': {
				outline: 'none'
			},
			'.cm-selectionBackground, ::selection': {
				backgroundColor: 'rgba(59, 130, 246, 0.18)'
			}
		});
	}

	$effect(() => {
		if (!host) return;
		const parent = host;

		// Build the editor once per mount. value/language/readOnly are applied
		// imperatively by the effects below so editing does not recreate the view.
		return untrack(() => {
			const saveKeymap = keymap.of([
				{
					key: 'Mod-s',
					run: () => {
						onSave?.();
						return true;
					}
				}
			]);

			const view = new EditorView({
				state: EditorState.create({
					doc: value,
					extensions: [
						basicSetup,
						EditorView.lineWrapping,
						editorTheme(),
						saveKeymap,
						languageCompartment.of(codemirrorLanguageSupport(language)),
						editableCompartment.of(EditorView.editable.of(!readOnly)),
						readOnlyCompartment.of(EditorState.readOnly.of(readOnly)),
						EditorView.updateListener.of((update) => {
							if (!update.docChanged) return;
							onChange?.(update.state.doc.toString());
						})
					]
				}),
				parent
			});

			editorView = view;

			return () => {
				if (editorView === view) editorView = null;
				view.destroy();
			};
		});
	});

	$effect(() => {
		const view = editorView;
		if (!view) return;
		view.dispatch({
			effects: [
				languageCompartment.reconfigure(codemirrorLanguageSupport(language)),
				editableCompartment.reconfigure(EditorView.editable.of(!readOnly)),
				readOnlyCompartment.reconfigure(EditorState.readOnly.of(readOnly))
			]
		});
	});

	$effect(() => {
		const view = editorView;
		if (!view) return;
		replaceEditorDocument(view, value);
	});

	$effect(() => {
		const view = editorView;
		const range = revealRange;
		if (!view || !range) return;
		const key = `${range.startLine}:${range.endLine}:${value.length}`;
		if (lastAppliedRevealKey === key) return;

		const doc = view.state.doc;
		if (doc.lines < 1) return;
		const startLine = Math.min(Math.max(1, range.startLine), doc.lines);
		const endLine = Math.min(Math.max(startLine, range.endLine), doc.lines);
		const from = doc.line(startLine).from;
		const to = doc.line(endLine).to;

		view.dispatch({
			selection: EditorSelection.single(from, to),
			effects: EditorView.scrollIntoView(from, { y: 'center' })
		});
		lastAppliedRevealKey = key;
		onRevealApplied?.();
	});
</script>

<div bind:this={host} class="file-editor"></div>

<style>
	.file-editor {
		width: 100%;
		height: 100%;
		min-height: 0;
	}
</style>
