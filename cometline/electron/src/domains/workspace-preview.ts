/** Permits browser and mail links only; native local handlers remain inaccessible to the renderer. */
export function isExternallyOpenableUrl(rawUrl: unknown) {
	try {
		const parsed = new URL(String(rawUrl));
		return ['http:', 'https:', 'mailto:'].includes(parsed.protocol);
	} catch {
		return false;
	}
}
