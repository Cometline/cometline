<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import type { CaretTrailSettings } from '$lib/types';
	import { faviconUrl, domainFromUrl, isHttpUrl } from '$lib/markdown/embed';
	import { openLink } from '$lib/open-link';

	let {
		value = $bindable(''),
		placeholder = '',
		ariaLabel = 'Message input',
		skillNames = [],
		caretTrail = { enabled: true, intensity: 0.72, speed: 0.68 },
		caretColor = '#72c0ff',
		onkeydown,
		onfiles
	}: {
		value?: string;
		placeholder?: string;
		ariaLabel?: string;
		skillNames?: string[];
		caretTrail?: CaretTrailSettings;
		caretColor?: string;
		onkeydown?: (e: KeyboardEvent) => void;
		onfiles?: (files: File[]) => void;
	} = $props();

	let wrap = $state<HTMLDivElement | null>(null);
	let editor = $state<HTMLDivElement | null>(null);
	let customCaret = $state<HTMLSpanElement | null>(null);
	let trailPoly = $state<SVGPolygonElement | null>(null);
	let focused = $state(false);
	// Guard so our own DOM writes don't recursively re-trigger input handling.
	let syncing = false;
	// IME composition guard — Enter during candidate selection must not trigger send.
	let composing = false;
	let raf = 0;
	// Guard so the temporary marker we insert to measure an empty line doesn't
	// re-enter measurement via selectionchange.
	let measuring = false;
	let caretReady = $state(false);
	// Caret geometry in wrap-local coordinates.
	let caretW = 2;
	let caretH = 22.5;
	// Trail smear model (mirrors cursor_tail.glsl): a quad is drawn between the
	// previous caret position (tail) and the current target (head); both ends
	// ease independently with easeOutCirc so the trail extends then collapses.
	let originX = 0; // tail anchor (where the smear starts from)
	let originY = 0;
	let targetX = 0; // head target (where the caret is heading)
	let targetY = 0;
	let animStart = 0; // performance.now() when the current move began
	let animating = false;

	let caretTrailEnabled = $derived(caretTrail.enabled);
	let baseTrailOpacity = $derived(0.32 + clampUnit(caretTrail.intensity) * 0.5);
	let caretStyle = $derived(`--rce-caret-color: ${caretColor}`);

	// easeOutCirc — matches the shader's chosen easing curve.
	function easeOutCirc(x: number): number {
		const c = clampUnit(x);
		return Math.sqrt(1 - (c - 1) * (c - 1));
	}

	// Move animation duration in ms, shorter when "speed" is high.
	function moveDuration(): number {
		return 90 + (1 - clampUnit(caretTrail.speed)) * 220;
	}

	// Vertical jump (in px) above which a move is treated as a teleport: the
	// caret snaps without a trail. We gate on vertical distance only — a one-line
	// word-wrap moves the caret a long way horizontally but only ~1 line down,
	// and should still animate; a click far away or programmatic jump spans
	// multiple lines and should snap clean.
	function maxTrailVerticalJump(): number {
		return caretH * 1.5;
	}

	// A move whose vertical delta exceeds ~half a line is a line crossing
	// (newline, word-wrap, selection extending down/up a line).
	function isLineCrossing(dy: number): boolean {
		return Math.abs(dy) > caretH * 0.5;
	}

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

	function clampUnit(value: number): number {
		if (!Number.isFinite(value)) return 0;
		return Math.min(1, Math.max(0, value));
	}

	function setCaretVisual(x: number, y: number) {
		if (!customCaret) return;
		customCaret.style.transform = `translate3d(${x}px, ${y}px, 0)`;
	}

	function clearTrail() {
		trailPoly?.setAttribute('points', '');
	}

	/**
	 * Measures the caret position in wrap-local coords. For a collapsed caret
	 * this is the caret itself; for a selection it's the *focus* end (the moving
	 * end driven by Shift+Arrow or mouse drag), so the comet rides the part of
	 * the selection the user is actively extending. The native blue highlight
	 * stays underneath. On an empty line getClientRects() returns nothing, so we
	 * temporarily insert a zero-width marker, measure it, then remove it.
	 * Returns null if the selection isn't inside the editor.
	 */
	function readCaretRect(): { x: number; y: number; h: number } | null {
		if (!wrap || !editor) return null;
		const selection = window.getSelection();
		if (!selection || selection.rangeCount === 0 || selection.focusNode == null) return null;
		const focusNode = selection.focusNode;
		if (focusNode !== editor && !editor.contains(focusNode)) return null;

		// Build a collapsed range at the selection's focus (moving) end.
		const range = document.createRange();
		try {
			range.setStart(focusNode, selection.focusOffset);
		} catch {
			return null;
		}
		range.collapse(true);

		const wrapRect = wrap.getBoundingClientRect();
		const lineHeight = Number.parseFloat(getComputedStyle(editor).lineHeight) || 22.5;

		let rect: DOMRect | undefined = range.getClientRects()[0];
		if (!rect || (rect.width === 0 && rect.height === 0)) {
			// Empty line / boundary: insert a zero-width marker to get a real rect.
			// We snapshot the live selection, probe, then restore it exactly so a
			// drag selection isn't collapsed by our measurement.
			measuring = true;
			const snap = {
				anchorNode: selection.anchorNode,
				anchorOffset: selection.anchorOffset,
				focusNode: selection.focusNode,
				focusOffset: selection.focusOffset
			};
			const marker = document.createElement('span');
			marker.textContent = '\u200b';
			const probe = range.cloneRange();
			probe.insertNode(marker);
			rect = marker.getBoundingClientRect();
			marker.remove();
			// Restore the original selection (anchor → focus) verbatim.
			if (snap.anchorNode && snap.focusNode) {
				try {
					selection.setBaseAndExtent(
						snap.anchorNode,
						snap.anchorOffset,
						snap.focusNode,
						snap.focusOffset
					);
				} catch {
					/* node may have been normalized away; ignore */
				}
			}
			measuring = false;
		}
		if (!rect) return null;
		return {
			x: rect.left - wrapRect.left,
			y: rect.top - wrapRect.top,
			h: rect.height || lineHeight
		};
	}

	function measureCaret() {
		if (!wrap || !editor || !caretTrailEnabled || !focused) return;
		const measured = readCaretRect();
		if (!measured) {
			// Selection focus is outside the editor or unmeasurable: keep the
			// caret where it is but stop drawing a trail.
			clearTrail();
			return;
		}

		caretH = measured.h;
		if (customCaret) customCaret.style.height = `${caretH}px`;

		if (!caretReady) {
			// First placement — snap, no trail.
			targetX = originX = measured.x;
			targetY = originY = measured.y;
			caretReady = true;
			setCaretVisual(targetX, targetY);
			clearTrail();
			return;
		}

		const dx = measured.x - targetX;
		const dy = measured.y - targetY;
		const dist = Math.hypot(dx, dy);
		if (dist < 0.5) return; // no meaningful move

		if (Math.abs(dy) > maxTrailVerticalJump()) {
			// Real teleport (click far away, multi-line programmatic jump): snap
			// without streaking a trail across the editor.
			targetX = originX = measured.x;
			targetY = originY = measured.y;
			setCaretVisual(targetX, targetY);
			clearTrail();
			return;
		}

		if (isLineCrossing(dy)) {
			// Line wrap / newline: the literal old→new path would slash a diagonal
			// across the editor. Instead, drop the comet in vertically — anchor the
			// tail directly above the new caret position (same X, one line up) so
			// the trail reads as a short vertical drop into the new line.
			originX = measured.x;
			originY = measured.y - dy;
		} else {
			// Same-line move: smear from the current head toward the new target.
			originX = targetX;
			originY = targetY;
		}
		targetX = measured.x;
		targetY = measured.y;
		animStart = performance.now();
		animating = true;
		if (!raf) raf = requestAnimationFrame(animateCaret);
	}

	function setTrailQuad(headX: number, headY: number, tailX: number, tailY: number, alpha: number) {
		if (!trailPoly) return;
		// Build a quad spanning the bar caret from the tail position to the head
		// position. The bar is `caretW` wide and `caretH` tall.
		const x0 = headX;
		const x1 = headX + caretW;
		const tx0 = tailX;
		const tx1 = tailX + caretW;
		const pts = [
			`${x0.toFixed(1)},${headY.toFixed(1)}`,
			`${x1.toFixed(1)},${headY.toFixed(1)}`,
			`${(tx1).toFixed(1)},${(tailY + caretH).toFixed(1)}`,
			`${(tx0).toFixed(1)},${(tailY + caretH).toFixed(1)}`
		];
		trailPoly.setAttribute('points', pts.join(' '));
		trailPoly.style.opacity = String(clampUnit(alpha) * baseTrailOpacity);
	}

	function animateCaret() {
		if (!animating) {
			raf = 0;
			return;
		}
		const now = performance.now();
		const duration = moveDuration();
		const progress = clampUnit((now - animStart) / duration);

		// Head leads, tail follows with a delay so the smear stretches then
		// collapses — same head/tail easing split as cursor_tail.glsl.
		const headEased = easeOutCirc(progress);
		const tailDelay = 0.18 + clampUnit(caretTrail.intensity) * 0.32;
		const tailEased = easeOutCirc(clampUnit((progress - tailDelay) / (1 - tailDelay)));

		const headX = originX + (targetX - originX) * headEased;
		const headY = originY + (targetY - originY) * headEased;
		const tailX = originX + (targetX - originX) * tailEased;
		const tailY = originY + (targetY - originY) * tailEased;

		setCaretVisual(headX, headY);

		const span = Math.hypot(headX - tailX, headY - tailY);
		if (span > 0.6) {
			// Fade the trail out as the move completes.
			setTrailQuad(headX, headY, tailX, tailY, 1 - progress * 0.35);
		} else {
			clearTrail();
		}

		if (progress >= 1) {
			animating = false;
			originX = targetX;
			originY = targetY;
			setCaretVisual(targetX, targetY);
			clearTrail();
			raf = 0;
			return;
		}
		raf = requestAnimationFrame(animateCaret);
	}

	function resetCaretTrail() {
		if (raf) cancelAnimationFrame(raf);
		raf = 0;
		caretReady = false;
		animating = false;
		clearTrail();
	}

	function scheduleCaretMeasure() {
		if (!caretTrailEnabled || measuring) return;
		requestAnimationFrame(measureCaret);
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

	function escapeRegex(value: string): string {
		return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	}

	function skillNameRegex() {
		const names = skillNames.map((name) => name.trim()).filter(Boolean);
		if (names.length === 0) return null;
		const pattern = names.sort((a, b) => b.length - a.length).map(escapeRegex).join('|');
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
		if (!(chip instanceof HTMLElement) || !chip.dataset.url) return;
		// A plain click on a chip opens its link.
		e.preventDefault();
		openLink(chip.dataset.url);
	}

	function onFocus() {
		focused = true;
		scheduleCaretMeasure();
	}

	function onBlur() {
		focused = false;
		resetCaretTrail();
	}

	function insertPlainText(text: string) {
		if (!editor) return;
		focus();
		document.execCommand('insertText', false, text);
		readValue();
		scheduleCaretMeasure();
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
		scheduleCaretMeasure();
	}

	export async function focusAsync() {
		await tick();
		focus();
	}

	export function insertText(text: string) {
		insertPlainText(text);
	}

	export function setText(text: string) {
		if (!editor) {
			value = text;
			return;
		}
		editor.textContent = text;
		value = text;
		focus();
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
		if (!caretTrailEnabled) {
			resetCaretTrail();
			return;
		}
		const onSelectionChange = () => scheduleCaretMeasure();
		const onResize = () => scheduleCaretMeasure();
		document.addEventListener('selectionchange', onSelectionChange);
		window.addEventListener('resize', onResize);
		scheduleCaretMeasure();
		return () => {
			document.removeEventListener('selectionchange', onSelectionChange);
			window.removeEventListener('resize', onResize);
			resetCaretTrail();
		};
	});

	onDestroy(resetCaretTrail);

	let isEmpty = $derived(value.trim() === '');
</script>

<div bind:this={wrap} class="rce-wrap" style={caretStyle}>
	{#if isEmpty}
		<div class="rce-placeholder" aria-hidden="true">{placeholder}</div>
	{/if}
	{#if caretTrailEnabled}
		<div class="rce-caret-layer" class:visible={focused && caretReady} aria-hidden="true">
			<svg class="rce-trail" focusable="false">
				<polygon bind:this={trailPoly}></polygon>
			</svg>
			<span bind:this={customCaret} class="rce-caret"></span>
		</div>
	{/if}
	<div
		bind:this={editor}
		class="rce-editor"
		class:trail-enabled={caretTrailEnabled}
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
		onfocus={onFocus}
		onblur={onBlur}
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
