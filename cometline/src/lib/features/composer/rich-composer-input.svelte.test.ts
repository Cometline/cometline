// @vitest-environment jsdom

import { describe, expect, it, vi } from 'vitest';
import { createRichComposerInputController } from './rich-composer-input.svelte';

function clipboardEvent(text: string | null): ClipboardEvent {
	const clipboardData = {
		files: [] as File[],
		getData: (type: string) => (type === 'text/plain' ? (text ?? '') : ''),
		setData: vi.fn()
	};
	return {
		clipboardData,
		preventDefault: vi.fn()
	} as unknown as ClipboardEvent;
}

function mount() {
	const editor = document.createElement('div');
	editor.contentEditable = 'true';
	document.body.append(editor);
	let value = '';
	const controller = createRichComposerInputController({
		getEditor: () => editor,
		setValue: (next) => {
			value = next;
		},
		getSkillNames: () => [],
		getMentionsEnabled: () => false,
		onkeydown: () => {},
		onfiles: () => {},
		onmentionquery: () => {}
	});
	return {
		editor,
		controller,
		get value() {
			return value;
		},
		cleanup: () => editor.remove()
	};
}

describe('createRichComposerInputController paste chips', () => {
	it('turns a large paste into a chip and keeps the text out of the layout', () => {
		const mounted = mount();
		const text = 'line one\r\nline two\r\nline three';
		const event = clipboardEvent(text);

		mounted.controller.onPaste(event);

		expect(event.preventDefault).toHaveBeenCalled();
		expect(mounted.editor.querySelector('.rce-paste-chip')).not.toBeNull();
		expect(mounted.editor.textContent).not.toContain('line one');
		expect(mounted.value).toContain('line one\nline two\nline three');
		mounted.cleanup();
	});

	it('keeps a chip collapsed on click and when restoring text', () => {
		const mounted = mount();
		mounted.controller.onPaste(clipboardEvent('a\nb\nc'));
		const chip = mounted.editor.querySelector('.rce-paste-chip');
		if (!(chip instanceof HTMLElement)) throw new Error('expected paste chip');
		const click = { target: chip, preventDefault: vi.fn() } as unknown as MouseEvent;

		mounted.controller.onEditorClick(click);

		expect(click.preventDefault).toHaveBeenCalled();
		expect(mounted.editor.querySelector('.rce-paste-chip')).not.toBeNull();
		expect(mounted.editor.textContent).not.toContain('a\nb\nc');

		const huge = 'y'.repeat(200);
		mounted.controller.setText(huge);
		expect(mounted.editor.querySelector('.rce-paste-chip')).not.toBeNull();
		expect(mounted.editor.textContent).not.toContain(huge.slice(0, 20));
		expect(mounted.value).toBe(huge);
		mounted.cleanup();
	});
});
