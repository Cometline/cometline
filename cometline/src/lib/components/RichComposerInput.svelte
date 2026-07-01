<script lang="ts">
	import { tick } from 'svelte';
	import type { CaretTrailSettings } from '$lib/types';
	import {
		customCaret,
		resetCustomCaret,
		scheduleCustomCaretMeasure,
		type CustomCaretState
	} from '$lib/dom/custom-caret';
	import { faviconUrl, domainFromUrl, isHttpUrl } from '$lib/markdown/embed';
	import { openLink } from '$lib/open-link';
	import { openWorkspaceFilePreview } from '$lib/workspace/open-file-preview';

	let {
		value = $bindable(''),
		placeholder = '',
		ariaLabel = 'Message input',
		skillNames = [],
		caretTrail = { enabled: true, intensity: 0.72, speed: 0.68 },
		caretColor = '#72c0ff',
		mentionsEnabled = true,
		onkeydown,
		onfiles,
		onmentionquery
	}: {
		value?: string;
		placeholder?: string;
		ariaLabel?: string;
		skillNames?: string[];
		caretTrail?: CaretTrailSettings;
		caretColor?: string;
		mentionsEnabled?: boolean;
		onkeydown?: (e: KeyboardEvent) => void;
		onfiles?: (files: File[]) => void;
		onmentionquery?: (payload: { query: string; active: boolean }) => void;
	} = $props();

	let wrap = $state<HTMLDivElement | null>(null);
	let editor = $state<HTMLDivElement | null>(null);
	let caretEl = $state<HTMLSpanElement | null>(null);
	let trailPoly = $state<SVGPolygonElement | null>(null);
	let focused = $state(false);
	let caretReady = $state(false);
	// Guard so our own DOM writes don't recursively re-trigger input handling.
	let syncing = false;
	// IME composition guard — Enter during candidate selection must not trigger send.
	let composing = false;
	// Mention state used by the parent composer to show/hide the file picker.
	let lastMentionActive = $state(false);
	let lastMentionQuery = $state('');

	let caretTrailEnabled = $derived(caretTrail.enabled);

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
					} else if (child.dataset.skillCommand) {
						out += child.dataset.skillCommand;
					} else if (child.dataset.filePath) {
						out += '@' + child.dataset.filePath;
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

	function scheduleCaretMeasure() {
		scheduleCustomCaretMeasure(editor);
	}

	function resetCaretTrail() {
		resetCustomCaret(editor);
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

	function makeSkillChip(name: string): HTMLElement {
		const chip = document.createElement('span');
		chip.className = 'rce-chip rce-skill-chip';
		chip.contentEditable = 'false';
		chip.dataset.skillCommand = `/${name}`;
		chip.title = `Use the ${name} skill`;

		const label = document.createElement('span');
		label.className = 'rce-chip-label';
		label.textContent = `/${name}`;
		chip.appendChild(label);
		return chip;
	}

	function makeFileChip(path: string): HTMLElement {
		const chip = document.createElement('span');
		chip.className = 'rce-chip rce-file-chip';
		chip.contentEditable = 'false';
		chip.dataset.filePath = path;
		chip.title = path;

		const label = document.createElement('span');
		label.className = 'rce-chip-label';
		label.textContent = '@' + path;
		chip.appendChild(label);
		return chip;
	}

	const mentionQueryChars = /^[a-zA-Z0-9_/.-]*$/;

	interface ActiveMention {
		query: string;
		range: Range;
	}

	function findActiveMention(): ActiveMention | null {
		if (!editor || !mentionsEnabled) return null;
		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0) return null;
		const focusNode = sel.focusNode;
		if (!focusNode || focusNode.nodeType !== Node.TEXT_NODE) return null;
		if (!editor.contains(focusNode)) return null;

		const text = focusNode.textContent ?? '';
		const offset = sel.focusOffset;

		let atIndex = -1;
		for (let i = offset - 1; i >= 0; i--) {
			const ch = text[i];
			if (ch === '@') {
				atIndex = i;
				break;
			}
			if (/\s/.test(ch)) break;
		}
		if (atIndex < 0) return null;

		// Require a word boundary before the '@' so email addresses don't trigger.
		if (atIndex > 0 && !/\s/.test(text[atIndex - 1])) return null;

		const query = text.slice(atIndex + 1, offset);
		if (!mentionQueryChars.test(query)) return null;

		const range = document.createRange();
		range.setStart(focusNode, atIndex);
		range.setEnd(focusNode, offset);
		return { query, range };
	}

	function updateMentionState() {
		if (!editor) return;
		const mention = findActiveMention();
		const active = mention !== null;
		const query = mention?.query ?? '';
		if (active !== lastMentionActive || query !== lastMentionQuery) {
			lastMentionActive = active;
			lastMentionQuery = query;
			onmentionquery?.({ query, active });
		}
	}

	function replaceRangeWithNodes(range: Range, nodes: Node[]) {
		range.deleteContents();
		const frag = document.createDocumentFragment();
		for (const node of nodes) {
			frag.appendChild(node);
		}
		range.insertNode(frag);
		range.collapse(false);
		const sel = window.getSelection();
		sel?.removeAllRanges();
		sel?.addRange(range);
	}

	function escapeRegex(value: string): string {
		return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	}

	function skillNameRegex() {
		const names = skillNames.map((name) => name.trim()).filter(Boolean);
		if (names.length === 0) return null;
		const pattern = names
			.sort((a, b) => b.length - a.length)
			.map(escapeRegex)
			.join('|');
		return new RegExp(`(^|\\s)\\/(${pattern})(?=\\s|$)`, 'g');
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

	function skillifyEditor(opts?: { allowCaretEnd?: boolean }) {
		const re = skillNameRegex();
		if (!editor || !re) return;
		const allowCaretEnd = opts?.allowCaretEnd ?? false;
		const walker = document.createTreeWalker(editor, NodeFilter.SHOW_TEXT);
		const textNodes: Text[] = [];
		let n = walker.nextNode();
		while (n) {
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
			re.lastIndex = 0;
			if (!re.test(text)) continue;

			const frag = document.createDocumentFragment();
			let cursor = 0;
			let replaced = false;
			re.lastIndex = 0;
			let match: RegExpExecArray | null;
			while ((match = re.exec(text)) !== null) {
				const prefix = match[1] ?? '';
				const name = match[2];
				const commandStart = match.index + prefix.length;
				const commandEnd = commandStart + name.length + 1;
				const caretInThisNode = caretNode === textNode;
				const caretAtCommandEnd = caretInThisNode && caretOffset === commandEnd;
				const hasTrailingBoundary = commandEnd < text.length && /\s/.test(text[commandEnd]);
				if (caretAtCommandEnd && !allowCaretEnd && !hasTrailingBoundary) continue;

				if (commandStart > cursor) {
					frag.appendChild(document.createTextNode(text.slice(cursor, commandStart)));
				}
				frag.appendChild(makeSkillChip(name));
				cursor = commandEnd;
				replaced = true;
			}
			if (!replaced) continue;
			if (cursor < text.length) frag.appendChild(document.createTextNode(text.slice(cursor)));

			textNode.replaceWith(frag);
			didChange = true;
		}
		if (didChange) {
			const range = document.createRange();
			range.selectNodeContents(editor);
			range.collapse(false);
			sel?.removeAllRanges();
			sel?.addRange(range);
		}
		return didChange;
	}

	function decorateEditor(opts?: { allowCaretEnd?: boolean }) {
		const didSkillify = skillifyEditor(opts);
		const didLinkify = linkifyEditor(opts);
		return didSkillify || didLinkify;
	}

	function onInput() {
		if (syncing || !editor) return;
		syncing = true;
		decorateEditor();
		syncing = false;
		readValue();
		scheduleCaretMeasure();
		updateMentionState();
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
		decorateEditor({ allowCaretEnd: true });
		syncing = false;
		readValue();
		scheduleCaretMeasure();
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
		if (composing || e.isComposing) return;
		onkeydown?.(e);
	}

	function onEditorClick(e: MouseEvent) {
		const target = e.target;
		if (!(target instanceof Element)) return;
		const chip = target.closest('.rce-chip');
		if (!(chip instanceof HTMLElement)) return;
		// A plain click on a file chip opens it in the side-panel editor.
		if (chip.dataset.filePath) {
			e.preventDefault();
			openWorkspaceFilePreview(chip.dataset.filePath);
			return;
		}
		// A plain click on a URL chip opens its link.
		if (chip.dataset.url) {
			e.preventDefault();
			openLink(chip.dataset.url);
		}
	}

	function insertPlainText(text: string) {
		if (!editor) return;
		focus();
		document.execCommand('insertText', false, text);
		readValue();
		scheduleCaretMeasure();
	}

	function setCaretPosition(atEnd: boolean) {
		if (!editor) return;
		const sel = window.getSelection();
		if (!sel) return;

		const range = document.createRange();
		const hasText = Boolean(editor.textContent);

		if (!hasText) {
			// Browsers often inject a lone <br> into empty contenteditables, which
			// makes a collapsed "end" caret render on a phantom second line.
			if (editor.innerHTML === '<br>') {
				editor.innerHTML = '';
			}
			range.setStart(editor, 0);
			range.collapse(true);
		} else if (atEnd) {
			range.selectNodeContents(editor);
			range.collapse(false);
		} else {
			const walker = document.createTreeWalker(editor, NodeFilter.SHOW_TEXT);
			const firstText = walker.nextNode();
			if (firstText) {
				range.setStart(firstText, 0);
				range.collapse(true);
			} else {
				range.setStart(editor, 0);
				range.collapse(true);
			}
		}

		sel.removeAllRanges();
		sel.addRange(range);
	}

	export function focus(options?: { position?: 'start' | 'end' }) {
		editor?.focus({ preventScroll: true });
		if (!editor) return;
		const atEnd =
			options?.position === 'end' ||
			(options?.position !== 'start' && Boolean(editor.textContent?.length));
		setCaretPosition(atEnd);
		scheduleCaretMeasure();
	}

	export async function focusAsync(options?: { position?: 'start' | 'end' }) {
		await tick();
		focus(options);
	}

	export function insertText(text: string) {
		insertPlainText(text);
	}

	export function insertFileMention(path: string) {
		if (!editor) return;
		const mention = findActiveMention();
		const chip = makeFileChip(path);
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
				editor.appendChild(chip);
				editor.appendChild(space);
				const endRange = document.createRange();
				endRange.selectNodeContents(editor);
				endRange.collapse(false);
				sel?.removeAllRanges();
				sel?.addRange(endRange);
			}
		}
		decorateEditor();
		syncing = false;
		readValue();
		scheduleCaretMeasure();
		updateMentionState();
	}

	export function getFilePaths(): string[] {
		if (!editor) return [];
		const chips = editor.querySelectorAll('.rce-file-chip');
		const paths: string[] = [];
		for (const chip of chips) {
			const path = (chip as HTMLElement).dataset.filePath;
			if (path) paths.push(path);
		}
		return paths;
	}

	export function setText(text: string) {
		if (!editor) {
			value = text;
			return;
		}
		editor.textContent = text;
		value = text;
		focus({ position: 'end' });
		syncing = true;
		decorateEditor({ allowCaretEnd: true });
		syncing = false;
		readValue();
		scheduleCaretMeasure();
	}

	/** Clears the editor (used after send). */
	export function clear() {
		if (editor) editor.innerHTML = '';
		value = '';
		resetCaretTrail();
		// Editor often keeps focus after send, but innerHTML = '' drops the selection.
		// Re-seat the caret and remeasure so the custom caret layer becomes visible.
		if (focused && editor) {
			editor.focus({ preventScroll: true });
			setCaretPosition(false);
			scheduleCaretMeasure();
		}
	}

	// Keep the DOM in sync when `value` is set externally to empty (e.g. cleared
	// after send). We only handle the clear case to avoid clobbering chips.
	$effect(() => {
		if (value === '' && editor && editor.textContent !== '') {
			editor.innerHTML = '';
		}
	});

	$effect(() => {
		const key = skillNames.join('\n');
		if (!editor || key === '') return;
		syncing = true;
		decorateEditor();
		syncing = false;
		readValue();
		scheduleCaretMeasure();
	});

	$effect(() => {
		const onSelectionChange = () => updateMentionState();
		document.addEventListener('selectionchange', onSelectionChange);
		return () => {
			document.removeEventListener('selectionchange', onSelectionChange);
		};
	});

	let isEmpty = $derived(value.trim() === '');
</script>

<div bind:this={wrap} class="rce-wrap">
	{#if isEmpty}
		<div class="rce-placeholder" aria-hidden="true">{placeholder}</div>
	{/if}
	{#if caretTrailEnabled}
		<div class="rce-caret-layer" class:visible={focused && caretReady} aria-hidden="true">
			<svg class="rce-trail" focusable="false">
				<polygon bind:this={trailPoly}></polygon>
			</svg>
			<span bind:this={caretEl} class="rce-caret"></span>
		</div>
	{/if}
	<div
		bind:this={editor}
		class="rce-editor scrollbar-none"
		class:trail-enabled={caretTrailEnabled}
		use:customCaret={{
			wrap,
			caret: caretEl,
			trail: trailPoly,
			caretTrail,
			color: caretColor,
			onStateChange: onCaretStateChange
		}}
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
		onfocus={updateMentionState}
		onblur={updateMentionState}
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

	.rce-editor.trail-enabled {
		caret-color: transparent;
	}

	.rce-caret-layer {
		position: absolute;
		inset: 0;
		pointer-events: none;
		z-index: 2;
		overflow: hidden;
		opacity: 0;
		transition: opacity 0.08s ease;
	}

	.rce-caret-layer.visible {
		opacity: 1;
	}

	.rce-trail {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		overflow: visible;
	}

	.rce-trail polygon {
		fill: var(--rce-caret-color);
		stroke: none;
		opacity: 0;
		filter: drop-shadow(0 0 6px var(--rce-caret-color));
	}

	@keyframes rce-caret-blink {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.75;
		}
	}

	.rce-caret {
		position: absolute;
		top: 0;
		left: 0;
		width: 2px;
		height: 1.5em;
		border-radius: 999px;
		background: var(--rce-caret-color);
		box-shadow: 0 0 9px var(--rce-caret-color);
		will-change: transform;
		animation: rce-caret-blink 1.1s ease-in-out infinite;
	}

	:global(.rce-caret.moving) {
		animation: none;
	}

	.rce-caret::after {
		content: '';
		position: absolute;
		inset: -5px -4px;
		border-radius: 999px;
		background: var(--rce-caret-color);
		opacity: 0.14;
		filter: blur(5px);
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

	.rce-editor :global(.rce-skill-chip) {
		border-color: rgba(37, 99, 235, 0.18);
		background: rgba(37, 99, 235, 0.06);
		color: #31517a;
		font-weight: 650;
	}

	.rce-editor :global(.rce-file-chip) {
		border-color: rgba(16, 185, 129, 0.22);
		background: rgba(16, 185, 129, 0.07);
		color: #1d5c42;
		font-weight: 650;
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
