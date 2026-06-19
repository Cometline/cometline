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
		`width:${to.width}px`
	].join(';');
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
export function dockUserOrigin(
	composerWrapper: DOMRect,
	target: DOMRect,
	gap = 12
): DOMRect {
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
		origin = 'textarea',
		onShowParticle,
		onHideParticle
	} = params;

	const reveal = () => {
		if (!deferReveal) revealStagedUser();
	};

	if (prefersReducedMotion()) {
		if (!skipOnPrepare) onPrepare?.();
		if (!skipStage) stageUser(text, images);
		await tick();
		scrollThreadToBottom(root);
		reveal();
		return true;
	}

	if (!skipOnPrepare) onPrepare?.();
	if (!skipStage) stageUser(text, images);
	await tick();
	scrollThreadToBottom(root);
	await afterPaint();

	const userTarget = await waitForSelector(root, '[data-flight-target="user"]');
	if (!(userTarget instanceof HTMLElement)) {
		reveal();
		return false;
	}

	const userTo = userTarget.getBoundingClientRect();
	let fromRect: DOMRect | null = null;

	if (origin === 'above-composer') {
		const composerWrapper = root.querySelector('.composer-wrapper');
		if (composerWrapper instanceof HTMLElement) {
			fromRect = dockUserOrigin(composerWrapper.getBoundingClientRect(), userTo);
		}
	} else {
		const textarea = root.querySelector('.composer .rce-editor');
		const capturedFrom =
			textareaFrom ?? (textarea instanceof HTMLElement ? textarea.getBoundingClientRect() : null);
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
