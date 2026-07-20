import { StickyThinkingVariant } from './sticky-thinking-variant';
import type { ThinkingIndicatorVariant } from './thinking-indicator';

/** Reactive sticky variant for the current turn-status phase. */
export function createStickyThinkingIndicator(getPhase: () => string | undefined) {
	const sticky = new StickyThinkingVariant();
	let variant = $state<ThinkingIndicatorVariant>('orbit');

	$effect(() => {
		const observedPhase = getPhase();
		const { variant: next, commitInMs } = sticky.observe(observedPhase);
		variant = next;
		if (commitInMs <= 0) return;

		const timer = setTimeout(() => {
			variant = sticky.observe(observedPhase).variant;
		}, commitInMs);
		return () => clearTimeout(timer);
	});

	return {
		get variant() {
			return variant;
		}
	};
}
