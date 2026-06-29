import type { Action } from 'svelte/action';

import type { CaretTrailSettings } from '$lib/types';
import { viewportDeltaToLocal } from '$lib/dom/caret-geometry';

const MEASURE_EVENT = 'customcaretmeasure';
const RESET_EVENT = 'customcaretreset';

export interface CustomCaretState {
	focused: boolean;
	ready: boolean;
}

export interface CustomCaretParams {
	wrap?: HTMLElement | null;
	caret?: HTMLSpanElement | null;
	trail?: SVGPolygonElement | null;
	caretTrail: CaretTrailSettings;
	color?: string;
	onStateChange?: (state: CustomCaretState) => void;
}

export function scheduleCustomCaretMeasure(node: HTMLElement | null) {
	node?.dispatchEvent(new CustomEvent(MEASURE_EVENT));
}

export function resetCustomCaret(node: HTMLElement | null) {
	node?.dispatchEvent(new CustomEvent(RESET_EVENT));
}

export const customCaret: Action<HTMLDivElement, CustomCaretParams> = (node, initialParams) => {
	let params = initialParams;
	let wrap = params.wrap ?? node.parentElement;
	let caret = params.caret ?? null;
	let trail = params.trail ?? null;
	let focused = false;
	let ready = false;
	let caretW = 2;
	let caretH = 22.5;
	let originX = 0;
	let originY = 0;
	let targetX = 0;
	let targetY = 0;
	let visualX = 0;
	let visualY = 0;
	let animStart = 0;
	let animating = false;
	let trailOnly = false;
	let measuring = false;
	let composing = false;
	let lastInputAt = 0;
	let raf = 0;

	function notifyState() {
		params.onStateChange?.({ focused, ready });
	}

	function clampUnit(value: number): number {
		if (!Number.isFinite(value)) return 0;
		return Math.min(1, Math.max(0, value));
	}

	function easeOutCirc(value: number): number {
		const clamped = clampUnit(value);
		return Math.sqrt(1 - (clamped - 1) * (clamped - 1));
	}

	function baseTrailOpacity(): number {
		return 0.32 + clampUnit(params.caretTrail.intensity) * 0.5;
	}

	function moveDuration(): number {
		return 90 + (1 - clampUnit(params.caretTrail.speed)) * 220;
	}

	function typingTrailDuration(): number {
		return 90 + (1 - clampUnit(params.caretTrail.speed)) * 110;
	}

	function maxTrailVerticalJump(): number {
		return caretH * 1.5;
	}

	function isLineCrossing(dy: number): boolean {
		return Math.abs(dy) > caretH * 0.5;
	}

	function isTypingMove(dy: number): boolean {
		return (composing || performance.now() - lastInputAt < 120) && !isLineCrossing(dy);
	}

	function clearTrail() {
		trail?.setAttribute('points', '');
	}

	function setCaretMoving(isMoving: boolean) {
		caret?.classList.toggle('moving', isMoving);
	}

	function setCaretVisual(x: number, y: number) {
		visualX = x;
		visualY = y;
		if (!caret) return;
		caret.style.transform = `translate3d(${x}px, ${y}px, 0)`;
	}

	function snapCaretTo(x: number, y: number) {
		if (raf) {
			cancelAnimationFrame(raf);
			raf = 0;
		}
		animating = false;
		trailOnly = false;
		targetX = originX = x;
		targetY = originY = y;
		setCaretVisual(x, y);
		setCaretMoving(false);
		clearTrail();
	}

	function resetCaretTrail() {
		if (raf) cancelAnimationFrame(raf);
		raf = 0;
		ready = false;
		animating = false;
		trailOnly = false;
		visualX = 0;
		visualY = 0;
		setCaretMoving(false);
		clearTrail();
		notifyState();
	}

	function readCaretRect(): { x: number; y: number; h: number } | null {
		if (!wrap) return null;
		const selection = window.getSelection();
		if (!selection || selection.rangeCount === 0 || selection.focusNode == null) return null;
		const focusNode = selection.focusNode;
		if (focusNode !== node && !node.contains(focusNode)) return null;

		const range = document.createRange();
		try {
			range.setStart(focusNode, selection.focusOffset);
		} catch {
			return null;
		}
		range.collapse(true);

		const lineHeight = Number.parseFloat(getComputedStyle(node).lineHeight) || 22.5;

		let rect: DOMRect | undefined = range.getClientRects()[0];
		if (!rect || (rect.width === 0 && rect.height === 0)) {
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
			if (snap.anchorNode && snap.focusNode) {
				try {
					selection.setBaseAndExtent(
						snap.anchorNode,
						snap.anchorOffset,
						snap.focusNode,
						snap.focusOffset
					);
				} catch {
					// Ignore restoration failures after DOM normalization.
				}
			}
			measuring = false;
		}
		if (!rect) return null;
		return viewportDeltaToLocal(wrap, rect, lineHeight);
	}

	function setTrailQuad(
		headX: number,
		headY: number,
		tailX: number,
		tailY: number,
		alpha: number
	) {
		if (!trail) return;
		const x0 = headX;
		const x1 = headX + caretW;
		const tx0 = tailX;
		const tx1 = tailX + caretW;
		const pts = [
			`${x0.toFixed(1)},${headY.toFixed(1)}`,
			`${x1.toFixed(1)},${headY.toFixed(1)}`,
			`${tx1.toFixed(1)},${(tailY + caretH).toFixed(1)}`,
			`${tx0.toFixed(1)},${(tailY + caretH).toFixed(1)}`
		];
		trail.setAttribute('points', pts.join(' '));
		trail.style.opacity = String(clampUnit(alpha) * baseTrailOpacity());
	}

	function animateCaret() {
		if (!animating) {
			raf = 0;
			return;
		}

		const now = performance.now();
		const duration = trailOnly ? typingTrailDuration() : moveDuration();
		const progress = clampUnit((now - animStart) / duration);

		let headX: number;
		let headY: number;
		let tailX: number;
		let tailY: number;

		if (trailOnly) {
			headX = targetX;
			headY = targetY;
			const tailEased = easeOutCirc(progress);
			tailX = originX + (targetX - originX) * tailEased;
			tailY = originY + (targetY - originY) * tailEased;
			setCaretVisual(headX, headY);
		} else {
			const headEased = easeOutCirc(progress);
			const tailDelay = 0.18 + clampUnit(params.caretTrail.intensity) * 0.32;
			const tailEased = easeOutCirc(clampUnit((progress - tailDelay) / (1 - tailDelay)));

			headX = originX + (targetX - originX) * headEased;
			headY = originY + (targetY - originY) * headEased;
			tailX = originX + (targetX - originX) * tailEased;
			tailY = originY + (targetY - originY) * tailEased;
			setCaretVisual(headX, headY);
		}

		const span = Math.hypot(headX - tailX, headY - tailY);
		if (span > 0.6) {
			setTrailQuad(headX, headY, tailX, tailY, 1 - progress * 0.35);
		} else {
			clearTrail();
		}

		if (progress >= 1) {
			animating = false;
			trailOnly = false;
			originX = targetX;
			originY = targetY;
			snapCaretTo(targetX, targetY);
			raf = 0;
			return;
		}

		raf = requestAnimationFrame(animateCaret);
	}

	function measureCaret() {
		if (!params.caretTrail.enabled || !focused) return;
		const measured = readCaretRect();
		if (!measured) {
			clearTrail();
			return;
		}

		caretH = measured.h;
		if (caret) caret.style.height = `${caretH}px`;

		if (!ready) {
			ready = true;
			notifyState();
			snapCaretTo(measured.x, measured.y);
			return;
		}

		const dx = measured.x - visualX;
		const dy = measured.y - visualY;
		const dist = Math.hypot(dx, dy);
		if (dist < 0.5) return;

		if (Math.abs(dy) > maxTrailVerticalJump()) {
			snapCaretTo(measured.x, measured.y);
			return;
		}

		const typing = isTypingMove(dy);
		trailOnly = typing;

		if (typing) {
			originX = visualX;
			originY = visualY;
			setCaretVisual(measured.x, measured.y);
		} else if (isLineCrossing(dy)) {
			originX = measured.x;
			originY = measured.y - dy;
		} else {
			originX = visualX;
			originY = visualY;
		}

		targetX = measured.x;
		targetY = measured.y;
		animStart = performance.now();
		animating = true;
		setCaretMoving(true);
		if (!raf) raf = requestAnimationFrame(animateCaret);
	}

	function scheduleCaretMeasure() {
		if (!params.caretTrail.enabled || measuring) return;
		requestAnimationFrame(measureCaret);
	}

	function syncRefs(next: CustomCaretParams) {
		params = next;
		wrap = params.wrap ?? node.parentElement;
		caret = params.caret ?? null;
		trail = params.trail ?? null;
	}

	function syncPresentation() {
		node.classList.toggle('trail-enabled', params.caretTrail.enabled);
		if (wrap) wrap.style.setProperty('--rce-caret-color', params.color ?? '#72c0ff');
		if (!params.caretTrail.enabled) {
			resetCaretTrail();
			return;
		}
		if (focused) scheduleCaretMeasure();
	}

	const onFocus = () => {
		focused = true;
		notifyState();
		scheduleCaretMeasure();
	};

	const onBlur = () => {
		focused = false;
		notifyState();
		resetCaretTrail();
	};

	const onInput = () => {
		lastInputAt = performance.now();
		scheduleCaretMeasure();
	};

	const onCompositionStart = () => {
		composing = true;
	};

	const onCompositionEnd = () => {
		setTimeout(() => {
			composing = false;
			lastInputAt = performance.now();
			scheduleCaretMeasure();
		}, 0);
	};

	const onSelectionChange = () => scheduleCaretMeasure();
	const onResize = () => scheduleCaretMeasure();
	const onScroll = () => scheduleCaretMeasure();
	const onMeasureEvent = () => scheduleCaretMeasure();
	const onResetEvent = () => resetCaretTrail();

	node.addEventListener('focus', onFocus);
	node.addEventListener('blur', onBlur);
	node.addEventListener('input', onInput);
	node.addEventListener('compositionstart', onCompositionStart);
	node.addEventListener('compositionend', onCompositionEnd);
	node.addEventListener('scroll', onScroll, { passive: true });
	node.addEventListener(MEASURE_EVENT, onMeasureEvent as EventListener);
	node.addEventListener(RESET_EVENT, onResetEvent as EventListener);
	document.addEventListener('selectionchange', onSelectionChange);
	window.addEventListener('resize', onResize);
	syncPresentation();
	notifyState();

	return {
		update(next) {
			syncRefs(next);
			syncPresentation();
		},
		destroy() {
			if (wrap) wrap.style.removeProperty('--rce-caret-color');
			node.classList.remove('trail-enabled');
			resetCaretTrail();
			node.removeEventListener('focus', onFocus);
			node.removeEventListener('blur', onBlur);
			node.removeEventListener('input', onInput);
			node.removeEventListener('compositionstart', onCompositionStart);
			node.removeEventListener('compositionend', onCompositionEnd);
			node.removeEventListener('scroll', onScroll);
			node.removeEventListener(MEASURE_EVENT, onMeasureEvent as EventListener);
			node.removeEventListener(RESET_EVENT, onResetEvent as EventListener);
			document.removeEventListener('selectionchange', onSelectionChange);
			window.removeEventListener('resize', onResize);
		}
	};
};
