export type IconVariant = 'default' | 'man';

export const ICON_VARIANT_OPTIONS: { id: IconVariant; label: string }[] = [
	{ id: 'default', label: 'Minako' },
	{ id: 'man', label: 'Souma' }
];

const VALID_ICON_VARIANTS = new Set<IconVariant>(ICON_VARIANT_OPTIONS.map((option) => option.id));

export function normalizeIconVariant(value: unknown): IconVariant {
	return value === 'man' ? 'man' : 'default';
}

export function isIconVariant(value: unknown): value is IconVariant {
	return typeof value === 'string' && VALID_ICON_VARIANTS.has(value as IconVariant);
}

export function projectAvatarSrc(variant: IconVariant, size: 96 | 192 | 384 = 192): string {
	const suffix = variant === 'man' ? '_man' : '';
	return `/project_avatar${suffix}_${size}.png`;
}

export function projectAvatarSrcset(variant: IconVariant): string {
	return ([96, 192, 384] as const)
		.map((size) => `${projectAvatarSrc(variant, size)} ${size}w`)
		.join(', ');
}

export function systemSoulFilename(variant: IconVariant): string {
	return variant === 'man' ? 'SOUL_MAN.md' : 'SOUL.md';
}
