import { tick } from 'svelte';

export const FLIGHT_MS = 560;
export const FLIGHT_EASE = 'cubic-bezier(0.22, 1, 0.36, 1)';

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

/** Origin rect for the user bubble — starts at the composer textarea. */
export function textareaUserOrigin(textarea: DOMRect, target: DOMRect): DOMRect {
	const width = target.width;
	const height = target.height;
	const left = Math.max(textarea.left, textarea.right - width);
	const top = textarea.top + (textarea.height - height) / 2;
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
	stageUser: (text: string) => void;
	revealStagedUser: () => void;
	onPrepare?: () => void;
	/** When true, `onPrepare` was already invoked by the caller. */
	skipOnPrepare?: boolean;
	/** Textarea rect captured before layout changes (first turn). */
	textareaFrom?: DOMRect | null;
	/** When true, the caller invokes `revealStagedUser` after coordinated animations. */
	deferReveal?: boolean;
	/** When true, the caller already staged the user bubble. */
	skipStage?: boolean;
	onShowParticle: (text: string, style: string) => void;
	onHideParticle: () => void;
}

/** Composer → thread user-bubble flight used on every send. */
export async function flyUserBubble(params: FlyUserBubbleParams): Promise<boolean> {
	const {
		root,
		text,
		stageUser,
		revealStagedUser,
		onPrepare,
		skipOnPrepare,
		textareaFrom,
		deferReveal,
		skipStage,
		onShowParticle,
		onHideParticle
	} = params;

	const reveal = () => {
		if (!deferReveal) revealStagedUser();
	};

	if (prefersReducedMotion()) {
		if (!skipOnPrepare) onPrepare?.();
		if (!skipStage) stageUser(text);
		await tick();
		scrollThreadToBottom(root);
		reveal();
		return true;
	}

	const textarea = root.querySelector('.composer textarea');
	const capturedFrom =
		textareaFrom ?? (textarea instanceof HTMLElement ? textarea.getBoundingClientRect() : null);

	if (!skipOnPrepare) onPrepare?.();
	if (!skipStage) stageUser(text);
	await tick();
	scrollThreadToBottom(root);
	await afterPaint();

	const userTarget = await waitForSelector(root, '[data-flight-target="user"]');
	if (!(userTarget instanceof HTMLElement) || !capturedFrom) {
		reveal();
		return false;
	}

	const userTo = userTarget.getBoundingClientRect();
	const style = translateStyle(textareaUserOrigin(capturedFrom, userTo), userTo);

	onShowParticle(text, style);
	await wait(FLIGHT_MS);
	onHideParticle();
	reveal();
	return true;
}
