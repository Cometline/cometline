<script lang="ts">
	import { tick } from 'svelte';
	import { faviconUrl, domainFromUrl, isHttpUrl } from '$lib/markdown/embed';
	import { openExternalLink } from '$lib/external-link';

	let {
		value = $bindable(''),
		placeholder = '',
		ariaLabel = 'Message input',
		onkeydown,
		onfiles
	}: {
		value?: string;
		placeholder?: string;
		ariaLabel?: string;
		onkeydown?: (e: KeyboardEvent) => void;
		onfiles?: (files: File[]) => void;
	} = $props();

	let editor = $state<HTMLDivElement | null>(null);
	// Guard so our own DOM writes don't recursively re-trigger input handling.
	let syncing = false;
	// IME composition guard — Enter during candidate selection must not trigger send.
	let composing = false;

	/**
	 * Serializes the contenteditable DOM back to plain text. URL chips serialize
	 * to their full URL (stored in data-url); <br>/block boundaries become \n.
	 */
	function serialize(root: HTMLElement): string {
		let out = '';
		const walk = (node: Node) => {
			for (const child of Array.from(node.childNodes)) {
				if (child.nodeType === Node.TEXT_NODE) {
					out += child.textContent ?? '';
				} else if (child instanceof HTMLElement) {
					if (child.dataset.url) {
						out += child.dataset.url;
					} else if (child.tagName === 'BR') {
						out += '\n';
					} else {
						const isBlock = /^(DIV|P)$/.test(child.tagName);
						if (isBlock && out && !out.endsWith('\n')) out += '\n';
						walk(child);
					}
				}
			}
		};
		walk(root);
		return out;
	}

	function readValue() {
		if (!editor) return;
		value = serialize(editor);
	}

	/** Build a non-editable inline chip element for a URL. */
	function makeChip(url: string): HTMLElement {
		const chip = document.createElement('span');
		chip.className = 'rce-chip';
		chip.contentEditable = 'false';
		chip.dataset.url = url;
		chip.title = url;

		const img = document.createElement('img');
		img.className = 'rce-chip-icon';
		img.src = faviconUrl(url);
		img.alt = '';
		img.width = 14;
		img.height = 14;
		img.addEventListener('error', () => (img.style.visibility = 'hidden'));

		const label = document.createElement('span');
		label.className = 'rce-chip-label';
		label.textContent = domainFromUrl(url);

		chip.appendChild(img);
		chip.appendChild(label);
		return chip;
	}

	/**
	 * Scans text nodes in the editor and replaces any complete bare URL with a
	 * chip. Only runs on text the user isn't actively typing the tail of (we
	 * require a trailing boundary char or that the URL isn't at the caret end).
	 */
	function linkifyEditor(opts?: { allowCaretEnd?: boolean }) {
		const allowCaretEnd = opts?.allowCaretEnd ?? false;
		if (!editor) return;
		const urlRe = /https?:\/\/[^\s<]+/g;
		const walker = document.createTreeWalker(editor, NodeFilter.SHOW_TEXT);
		const textNodes: Text[] = [];
		let n = walker.nextNode();
		while (n) {
			// Skip text already inside a chip.
			if (!(n.parentElement && n.parentElement.closest('.rce-chip'))) {
				textNodes.push(n as Text);
			}
			n = walker.nextNode();
		}

		const sel = window.getSelection();
		const caretNode = sel && sel.rangeCount > 0 ? sel.focusNode : null;
		const caretOffset = sel && sel.rangeCount > 0 ? sel.focusOffset : 0;

		let didChange = false;
		for (const textNode of textNodes) {
			const text = textNode.textContent ?? '';
			urlRe.lastIndex = 0;
			let match: RegExpExecArray | null = urlRe.exec(text);
			if (!match) continue;

			// Build replacement fragment.
			const frag = document.createDocumentFragment();
			let cursor = 0;
			let replaced = false;
			urlRe.lastIndex = 0;
			while ((match = urlRe.exec(text)) !== null) {
				const start = match.index;
				const end = start + match[0].length;
				// Don't chipify a URL the caret is still typing at the end of —
				// unless this is a paste, where we chipify immediately.
				const caretInThisNode = caretNode === textNode;
				const caretAtUrlEnd = caretInThisNode && caretOffset === end;
				if (caretAtUrlEnd && !allowCaretEnd) continue;
				let url = match[0];
				const trailing = /[.,;:!?)\]}'"]+$/.exec(url);
				if (trailing) url = url.slice(0, url.length - trailing[0].length);
				if (!isHttpUrl(url)) continue;

				if (start > cursor)
					frag.appendChild(document.createTextNode(text.slice(cursor, start)));
				frag.appendChild(makeChip(url));
				const suffix = match[0].slice(url.length);
				if (suffix) frag.appendChild(document.createTextNode(suffix));
				cursor = end;
				replaced = true;
			}
			if (!replaced) continue;
			if (cursor < text.length) frag.appendChild(document.createTextNode(text.slice(cursor)));

			// Append a trailing space + place caret after the inserted content so
			// typing continues normally after a chip.
			const trailingSpace = document.createTextNode('\u00a0');
			frag.appendChild(trailingSpace);
			textNode.replaceWith(frag);
			didChange = true;

			// Restore caret to just after the trailing space.
			const range = document.createRange();
			range.setStartAfter(trailingSpace);
			range.collapse(true);
			sel?.removeAllRanges();
			sel?.addRange(range);
		}
		return didChange;
	}

	function onInput() {
		if (syncing || !editor) return;
		syncing = true;
		linkifyEditor();
		syncing = false;
		readValue();
	}

	function onPaste(e: ClipboardEvent) {
		const files = Array.from(e.clipboardData?.files ?? []).filter((file) =>
			file.type.startsWith('image/')
		);
		if (files.length > 0) {
			e.preventDefault();
			onfiles?.(files);
			return;
		}

		// Force plain-text paste so we don't inherit foreign HTML, then linkify
		// immediately so a pasted URL becomes a chip without needing an extra
		// keystroke.
		const text = e.clipboardData?.getData('text/plain');
		if (text == null) return;
		e.preventDefault();
		document.execCommand('insertText', false, text);
		if (!editor) return;
		syncing = true;
		linkifyEditor({ allowCaretEnd: true });
		syncing = false;
		readValue();
	}

	function onCompositionStart() {
		composing = true;
	}

	function onCompositionEnd() {
		// Defer reset: some browsers fire the confirming keydown after compositionend.
		setTimeout(() => {
			composing = false;
		}, 0);
	}

	function onKeydownInternal(e: KeyboardEvent) {
		if (composing || e.isComposing) return;
		onkeydown?.(e);
	}

	function onEditorClick(e: MouseEvent) {
		const target = e.target;
		if (!(target instanceof Element)) return;
		const chip = target.closest('.rce-chip');
		if (!(chip instanceof HTMLElement) || !chip.dataset.url) return;
		// A plain click on a chip opens its link.
		e.preventDefault();
		openExternalLink(chip.dataset.url);
	}

	function insertPlainText(text: string) {
		if (!editor) return;
		focus();
		document.execCommand('insertText', false, text);
		readValue();
	}

	export function focus() {
		editor?.focus({ preventScroll: true });
		// Move caret to end.
		if (editor) {
			const range = document.createRange();
			range.selectNodeContents(editor);
			range.collapse(false);
			const sel = window.getSelection();
			sel?.removeAllRanges();
			sel?.addRange(range);
		}
	}

	export async function focusAsync() {
		await tick();
		focus();
	}

	export function insertText(text: string) {
		insertPlainText(text);
	}

	/** Clears the editor (used after send). */
	export function clear() {
		if (editor) editor.innerHTML = '';
		value = '';
	}

	// Keep the DOM in sync when `value` is set externally to empty (e.g. cleared
	// after send). We only handle the clear case to avoid clobbering chips.
	$effect(() => {
		if (value === '' && editor && editor.textContent !== '') {
			editor.innerHTML = '';
		}
	});

	let isEmpty = $derived(value.trim() === '');
</script>

<div class="rce-wrap">
	{#if isEmpty}
		<div class="rce-placeholder" aria-hidden="true">{placeholder}</div>
	{/if}
	<div
		bind:this={editor}
		class="rce-editor"
		contenteditable="true"
		role="textbox"
		tabindex="0"
		aria-multiline="true"
		aria-label={ariaLabel}
		oninput={onInput}
		onpaste={onPaste}
		onkeydown={onKeydownInternal}
		oncompositionstart={onCompositionStart}
		oncompositionend={onCompositionEnd}
		onclick={onEditorClick}
	></div>
</div>

<style>
	.rce-wrap {
		position: relative;
		width: 100%;
	}

	.rce-placeholder {
		position: absolute;
		inset: 0;
		pointer-events: none;
		color: var(--text-soft);
		font-size: 15px;
		line-height: 1.5;
		white-space: pre-wrap;
	}

	.rce-editor {
		width: 100%;
		min-height: calc(1.5em * 3);
		max-height: calc(1.5em * 8);
		overflow-y: auto;
		font-size: 15px;
		line-height: 1.5;
		color: var(--text-main);
		outline: none;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: inherit;
	}

	.rce-editor :global(.rce-chip) {
		display: inline-flex;
		align-items: center;
		gap: 0.3em;
		max-width: 16rem;
		vertical-align: middle;
		padding: 0.1em 0.45em;
		margin: 0 2px;
		border: 1px solid var(--border-soft);
		border-radius: 6px;
		background: rgba(15, 23, 42, 0.04);
		font-size: 0.92em;
		line-height: 1.3;
		color: var(--text-muted);
		white-space: nowrap;
		user-select: none;
		cursor: pointer;
	}

	.rce-editor :global(.rce-chip-icon) {
		flex-shrink: 0;
		width: 1em;
		height: 1em;
		object-fit: contain;
		border-radius: 3px;
	}

	.rce-editor :global(.rce-chip-label) {
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
