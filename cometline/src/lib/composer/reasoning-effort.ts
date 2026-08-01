/**
 * Reasoning effort cycling for the composer shortcut.
 *
 * Effort cycles through: auto (empty) -> first supported option -> ... ->
 * last supported option -> auto. Values not in the supported list (or an
 * empty list) reset to the first supported option; an unsupported model
 * yields no change.
 */
export function nextReasoningEffort(current: string, supported: string[]): string {
	if (supported.length === 0) return current;
	const cycle = ['', ...supported];
	const index = cycle.indexOf(current);
	const next = cycle[(index + 1) % cycle.length];
	return next === undefined ? '' : next;
}
