import type { Action } from 'svelte/action';

import type { CaretTrailSettings } from '$lib/types';
import { viewportDeltaToLocal } from '$lib/dom/caret-geometry';
import {
	clampUnit,
	easeOutCirc,
	pointsToSvg,
	trailPolygonPoints
} from '$lib/dom/caret-trail-geometry';

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

export type CaretMotionMode = 'typingTrail' | 'fullMove';

/** Same-line typing → head snap + trail; wrap / arrows / selection → slide caret only. */
export function resolveCaretMotion(opts: {
	dy: number;
	caretH: number;
	recentlyTyped: boolean;
}): CaretMotionMode {
	const lineCrossing = Math.abs(opts.dy) > opts.caretH * 0.5;
	return opts.recentlyTyped && !lineCrossing ? 'typingTrail' : 'fullMove';
}

function lerp(a: number, b: number, t: number): number {
	return a + (b - a) * t;
}

export const customCaret: Action<HTMLDivElement, CustomCaretParams> = (node, initialParams) => {
	let params = initialParams;
	let wrap = params.wrap ?? node.parentElement;
	let caret = params.caret ?? null;
	let trail = params.trail ?? null;
	let focused = false;
	let ready = false;
	const caretW = 2;
	let caretH = 22.5;
	let originX = 0;
	let originY = 0;
	let targetX = 0;
	let targetY = 0;
	let visualX = 0;
	let visualY = 0;
	let animStart = 0;
	let animating = false;
	let motionMode: CaretMotionMode = 'fullMove';
	let measuring = false;
	let composing = false;
	let lastInputAt = 0;
	let raf = 0;
	let measureRaf = 0;

	function notifyState() {
		params.onStateChange?.({ focused, ready });
	}

	function baseTrailOpacity(): number {
		return 0.32 + clampUnit(params.caretTrail.intensity) * 0.5;
	}

	function animDuration(mode: CaretMotionMode): number {
		const span = mode === 'typingTrail' ? 110 : 220;
		return 90 + (1 - clampUnit(params.caretTrail.speed)) * span;
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

	function cancelFrames() {
		if (measureRaf) {
			cancelAnimationFrame(measureRaf);
			measureRaf = 0;
		}
		if (raf) {
			cancelAnimationFrame(raf);
			raf = 0;
		}
	}

	function snapCaretTo(x: number, y: number) {
		cancelFrames();
		animating = false;
		motionMode = 'fullMove';
		targetX = originX = x;
		targetY = originY = y;
		setCaretVisual(x, y);
		setCaretMoving(false);
		clearTrail();
	}

	function resetCaretTrail() {
		cancelFrames();
		ready = false;
		animating = false;
		motionMode = 'fullMove';
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

	function setTrailQuad(headX: number, headY: number, tailX: number, tailY: number, alpha: number) {
		if (!trail) return;
		const points = trailPolygonPoints(
			{ x: headX, y: headY },
			{ x: tailX, y: tailY },
			caretW,
			caretH
		);
		trail.setAttribute('points', pointsToSvg(points));
		trail.style.opacity = String(clampUnit(alpha) * baseTrailOpacity());
	}

	function animateCaret() {
		if (!animating) {
			raf = 0;
			return;
		}

		const trailOnly = motionMode === 'typingTrail';
		const progress = clampUnit((performance.now() - animStart) / animDuration(motionMode));

		if (trailOnly) {
			const t = easeOutCirc(progress);
			const tailX = lerp(originX, targetX, t);
			const tailY = lerp(originY, targetY, t);
			setCaretVisual(targetX, targetY);
			const span = Math.hypot(targetX - tailX, targetY - tailY);
			if (span > 0.6) {
				setTrailQuad(targetX, targetY, tailX, tailY, 1 - progress * 0.35);
			} else {
				clearTrail();
			}
		} else {
			// fullMove: slide the caret only — no trail smear.
			const t = easeOutCirc(progress);
			setCaretVisual(lerp(originX, targetX, t), lerp(originY, targetY, t));
			clearTrail();
		}

		if (progress >= 1) {
			snapCaretTo(targetX, targetY);
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

		const dy = measured.y - visualY;
		if (Math.hypot(measured.x - visualX, dy) < 0.5) return;

		const recentlyTyped = composing || performance.now() - lastInputAt < 120;
		motionMode = resolveCaretMotion({ dy, caretH, recentlyTyped });

		originX = visualX;
		originY = visualY;
		if (motionMode === 'typingTrail') {
			setCaretVisual(measured.x, measured.y);
		}

		targetX = measured.x;
		targetY = measured.y;
		animStart = performance.now();
		animating = true;
		setCaretMoving(true);
		if (!raf) raf = requestAnimationFrame(animateCaret);
	}

	function scheduleCaretMeasure() {
		if (!params.caretTrail.enabled || measuring || measureRaf) return;
		measureRaf = requestAnimationFrame(() => {
			measureRaf = 0;
			measureCaret();
		});
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
