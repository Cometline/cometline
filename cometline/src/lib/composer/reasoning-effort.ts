/**
 * Reasoning effort cycling for the composer shortcut.
 *
 * Effort cycles through: auto (empty) -> first supported option -> ... ->
 * last supported option -> auto. A value outside the supported list resets
 * to auto; an unsupported model yields no change.
 */
export function nextReasoningEffort(current: string, supported: string[]): string {
	if (supported.length === 0) return current;
	const cycle = ['', ...supported];
	const index = cycle.indexOf(current);
	const next = cycle[(index + 1) % cycle.length];
	return next === undefined ? '' : next;
}
