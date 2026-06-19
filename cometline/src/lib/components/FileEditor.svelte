<script lang="ts">
	import { untrack } from 'svelte';
	import { basicSetup, EditorView } from 'codemirror';
	import { Compartment, EditorState } from '@codemirror/state';
	import { keymap } from '@codemirror/view';
	import type { Extension } from '@codemirror/state';
	import { codemirrorLanguageSupport } from '$lib/workspace/codemirror-language';

	let {
		value,
		language,
		readOnly = false,
		onChange,
		onSave
	}: {
		value: string;
		language: string | null;
		readOnly?: boolean;
		onChange?: (value: string) => void;
		onSave?: () => void;
	} = $props();

	let host = $state<HTMLDivElement | null>(null);
	let editorView = $state<EditorView | null>(null);
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
		const current = view.state.doc.toString();
		if (current === value) return;
		view.dispatch({
			changes: { from: 0, to: view.state.doc.length, insert: value }
		});
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
