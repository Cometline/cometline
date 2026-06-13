export const FLIGHT_MS = 560;
export const FLIGHT_EASE = 'cubic-bezier(0.22, 1, 0.36, 1)';

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

/** Origin rect for the user bubble — starts at the composer textarea. */
export function textareaUserOrigin(textarea: DOMRect, target: DOMRect): DOMRect {
	const width = Math.max(target.width, Math.min(520, Math.max(56, textarea.width * 0.45)));
	const height = Math.max(target.height, 40);
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
