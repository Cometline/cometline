import { tick } from 'svelte';
import type { ImageAttachment } from '$lib/types';

export const FLIGHT_MS = 560;
export const FLIGHT_EASE = 'cubic-bezier(0.22, 1, 0.36, 1)';
/** Fraction of horizontal travel from composer → thread (lower = shorter slide-in). */
export const USER_BUBBLE_FLIGHT_HORIZONTAL_BLEND = 0.35;
/** Fraction of vertical travel from composer → thread (lower = shorter slide-in). */
export const USER_BUBBLE_FLIGHT_VERTICAL_BLEND = 0.35;

export interface BlendFlightOriginOptions {
	horizontalBlend?: number;
	verticalBlend?: number;
}

export function prefersReducedMotion(): boolean {
	return (
		typeof window !== 'undefined' &&
		window.matchMedia('(prefers-reduced-motion: reduce)').matches
	);
}

export function scrollThreadToBottom(root: ParentNode): void {
	const scroller = root.querySelector('.thread');
	if (scroller instanceof HTMLElement) {
		scroller.scrollTo({ top: scroller.scrollHeight, behavior: 'auto' });
	}
}

export function rectStyle(from: DOMRect, to: DOMRect): string {
	const dx = to.left - from.left;
	const dy = to.top - from.top;
	const sx = to.width / Math.max(from.width, 1);
	const sy = to.height / Math.max(from.height, 1);
	return [
		`--flight-x:${dx}px`,
		`--flight-y:${dy}px`,
		`--flight-sx:${sx}`,
		`--flight-sy:${sy}`,
		`left:${from.left}px`,
		`top:${from.top}px`,
		`width:${from.width}px`,
		`height:${from.height}px`
	].join(';');
}

export function translateStyle(from: DOMRect, to: DOMRect): string {
	const dx = to.left - from.left;
	const dy = to.top - from.top;
	return [
		`--flight-x:${dx}px`,
		`--flight-y:${dy}px`,
		`left:${from.left}px`,
		`top:${from.top}px`,
		`width:${to.width}px`,
		`height:${to.height}px`,
		`box-sizing:border-box`
	].join(';');
}

export interface RectTransform {
	dx: number;
	dy: number;
	sx: number;
	sy: number;
}

export function rectTransform(from: DOMRect, to: DOMRect): RectTransform {
	return {
		dx: from.left - to.left,
		dy: from.top - to.top,
		sx: from.width / Math.max(to.width, 1),
		sy: from.height / Math.max(to.height, 1)
	};
}

/** Wait for an element's CSS animation, with a timeout fallback. */
export async function waitForAnimationEnd(
	element: HTMLElement | null,
	extraMs = 160
): Promise<void> {
	await tick();
	if (!element) return;

	await new Promise<void>((resolve) => {
		let done = false;
		const finish = () => {
			if (done) return;
			done = true;
			element.removeEventListener('animationend', finish);
			window.clearTimeout(timeout);
			resolve();
		};
		const timeout = window.setTimeout(finish, FLIGHT_MS + extraMs);
		element.addEventListener('animationend', finish, { once: true });
	});
}

/** Measure a layout rect after it stops changing for two paint cycles. */
export async function measureStableRect(element: HTMLElement): Promise<DOMRect> {
	let rect = element.getBoundingClientRect();
	let stableFrames = 0;

	for (let frame = 0; frame < 40; frame++) {
		await afterPaint();
		const next = element.getBoundingClientRect();
		const stable =
			Math.abs(next.left - rect.left) < 0.5 &&
			Math.abs(next.top - rect.top) < 0.5 &&
			Math.abs(next.width - rect.width) < 0.5 &&
			Math.abs(next.height - rect.height) < 0.5;

		if (stable) {
			stableFrames += 1;
			if (stableFrames >= 2) return next;
		} else {
			stableFrames = 0;
		}

		rect = next;
	}

	return rect;
}

/** Animate an element from its previous rect to its current layout rect. */
export async function animateElementToRect(
	element: HTMLElement,
	from: DOMRect | null
): Promise<void> {
	if (!from || prefersReducedMotion()) return;

	await afterPaint();
	const to = element.getBoundingClientRect();
	const { dx, dy, sx, sy } = rectTransform(from, to);

	element.style.transition = 'none';
	element.style.transformOrigin = 'top left';
	element.style.transform = `translate3d(${dx}px, ${dy}px, 0) scale(${sx}, ${sy})`;
	void element.offsetWidth;
	await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

	element.style.transition = `transform ${FLIGHT_MS}ms var(--ease-smooth)`;
	element.style.transform = 'translate3d(0, 0, 0)';

	await new Promise<void>((resolve) => {
		let done = false;
		const finish = () => {
			if (done) return;
			done = true;
			element.removeEventListener('transitionend', onEnd);
			window.clearTimeout(timeout);
			element.style.transition = '';
			element.style.transform = '';
			element.style.transformOrigin = '';
			resolve();
		};
		const onEnd = (event: TransitionEvent) => {
			if (event.propertyName === 'transform') finish();
		};
		const timeout = window.setTimeout(finish, FLIGHT_MS + 80);
		element.addEventListener('transitionend', onEnd);
	});
}

const LAYOUT_STABLE_FRAMES = 2;
const MAX_LAYOUT_WAIT_FRAMES = 24;

/** Wait until `.user-stack` width stops changing, then measure the hidden bubble. */
export async function measureStableUserBubble(target: HTMLElement): Promise<DOMRect> {
	const stack = target.closest('.user-stack');
	let prevWidth = -1;
	let stableFrames = 0;

	for (let frame = 0; frame < MAX_LAYOUT_WAIT_FRAMES; frame++) {
		await afterPaint();
		const stackWidth = stack instanceof HTMLElement ? stack.getBoundingClientRect().width : 0;
		if (stackWidth > 0 && stackWidth === prevWidth) {
			stableFrames++;
			if (stableFrames >= LAYOUT_STABLE_FRAMES) {
				return target.getBoundingClientRect();
			}
		} else {
			stableFrames = 0;
			prevWidth = stackWidth;
		}
	}

	return target.getBoundingClientRect();
}

/** Pull the flight origin closer to the target so the bubble does not slide as far. */
export function blendFlightOrigin(
	from: DOMRect,
	to: DOMRect,
	options: BlendFlightOriginOptions = {}
): DOMRect {
	const horizontalBlend = options.horizontalBlend ?? USER_BUBBLE_FLIGHT_HORIZONTAL_BLEND;
	const verticalBlend = options.verticalBlend ?? USER_BUBBLE_FLIGHT_VERTICAL_BLEND;
	const left = to.left + (from.left - to.left) * horizontalBlend;
	const top = to.top + (from.top - to.top) * verticalBlend;
	return new DOMRect(left, top, from.width, from.height);
}

export type UserBubbleFlightOrigin = 'textarea' | 'above-composer';

/** Origin rect for the user bubble — starts at the composer textarea. */
export function textareaUserOrigin(textarea: DOMRect, target: DOMRect): DOMRect {
	const width = target.width;
	const height = target.height;
	const left = Math.max(textarea.left, textarea.right - width);
	const top = textarea.top + (textarea.height - height) / 2;
	return new DOMRect(left, top, width, height);
}

/** Origin rect for docked follow-up turns — starts just above the composer shell. */
export function dockUserOrigin(composerWrapper: DOMRect, target: DOMRect, gap = 12): DOMRect {
	const width = target.width;
	const height = target.height;
	const left = Math.max(composerWrapper.left, composerWrapper.right - width);
	const top = composerWrapper.top - gap - height;
	return new DOMRect(left, top, width, height);
}

export function wait(ms: number): Promise<void> {
	return new Promise((resolve) => window.setTimeout(resolve, ms));
}

/** Wait until the browser has painted pending DOM updates. */
export function afterPaint(): Promise<void> {
	return new Promise((resolve) => {
		requestAnimationFrame(() => {
			requestAnimationFrame(() => resolve());
		});
	});
}

export async function waitForSelector(
	root: ParentNode,
	selector: string,
	timeoutMs = 4000
): Promise<Element | null> {
	const existing = root.querySelector(selector);
	if (existing) return existing;

	return new Promise((resolve) => {
		const started = performance.now();
		const tick = () => {
			const found = root.querySelector(selector);
			if (found) {
				resolve(found);
				return;
			}
			if (performance.now() - started >= timeoutMs) {
				resolve(null);
				return;
			}
			requestAnimationFrame(tick);
		};
		requestAnimationFrame(tick);
	});
}

export interface FlyUserBubbleParams {
	root: HTMLElement;
	text: string;
	images?: ImageAttachment[];
	stageUser: (text: string, images?: ImageAttachment[]) => void;
	revealStagedUser: () => void;
	onPrepare?: () => void;
	/** When true, `onPrepare` was already invoked by the caller. */
	skipOnPrepare?: boolean;
	/** Textarea rect captured before layout changes (first turn). */
	textareaFrom?: DOMRect | null;
	/** When true, the caller invokes `revealStagedUser` after coordinated animations. */
	deferReveal?: boolean;
	/** When true, never call `revealStagedUser` (caller owns reveal). */
	skipReveal?: boolean;
	/** When true, the caller dismisses the particle after the thread handoff. */
	deferHideParticle?: boolean;
	/** When true, the caller already staged the user bubble. */
	skipStage?: boolean;
	/** First turn uses the textarea; follow-up turns launch above the docked composer. */
	origin?: UserBubbleFlightOrigin;
	onShowParticle: (text: string, images: ImageAttachment[] | undefined, style: string) => void;
	onHideParticle: () => void;
}

/** Composer → thread user-bubble flight used on every send. */
export async function flyUserBubble(params: FlyUserBubbleParams): Promise<boolean> {
	const {
		root,
		text,
		images,
		stageUser,
		revealStagedUser,
		onPrepare,
		skipOnPrepare,
		textareaFrom,
		deferReveal,
		deferHideParticle,
		skipStage,
		skipReveal,
		origin = 'textarea',
		onShowParticle,
		onHideParticle
	} = params;

	const reveal = () => {
		if (skipReveal || deferReveal) return;
		revealStagedUser();
	};
	const isFollowUpTurn = origin === 'above-composer';

	if (prefersReducedMotion()) {
		if (!skipOnPrepare) onPrepare?.();
		if (!skipStage) stageUser(text, images);
		await tick();
		if (!isFollowUpTurn) scrollThreadToBottom(root);
		reveal();
		return true;
	}

	if (!skipOnPrepare) onPrepare?.();
	if (!skipStage) stageUser(text, images);
	await tick();
	if (!isFollowUpTurn) scrollThreadToBottom(root);
	await afterPaint();

	const userTarget = await waitForSelector(root, '[data-flight-target="user"]');
	if (!(userTarget instanceof HTMLElement)) {
		reveal();
		return false;
	}

	const userTo = await measureStableUserBubble(userTarget);
	let fromRect: DOMRect | null = null;

	if (origin === 'above-composer') {
		const composerWrapper = root.querySelector('.composer-wrapper');
		if (composerWrapper instanceof HTMLElement) {
			fromRect = dockUserOrigin(composerWrapper.getBoundingClientRect(), userTo);
		}
	} else {
		const textarea = root.querySelector('.composer .rce-editor');
		const capturedFrom =
			textareaFrom ??
			(textarea instanceof HTMLElement ? textarea.getBoundingClientRect() : null);
		if (capturedFrom) {
			fromRect = textareaUserOrigin(capturedFrom, userTo);
		}
	}

	if (!fromRect) {
		reveal();
		return false;
	}

	fromRect = blendFlightOrigin(fromRect, userTo);

	const style = translateStyle(fromRect, userTo);

	onShowParticle(text, images, style);
	await wait(FLIGHT_MS);
	if (!deferHideParticle) onHideParticle();
	reveal();
	return true;
}
