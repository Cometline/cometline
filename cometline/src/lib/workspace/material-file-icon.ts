/**
 * Resolve Material Icon Theme icons for file paths (same set as the popular
 * VS Code extension). Light-theme overrides are preferred for Cometline's UI.
 */
import manifest from 'material-icon-theme/dist/material-icons.json';

type IconManifest = {
	file?: string;
	fileNames?: Record<string, string>;
	fileExtensions?: Record<string, string>;
	light?: {
		fileNames?: Record<string, string>;
		fileExtensions?: Record<string, string>;
	};
};

const icons = manifest as IconManifest;

/** Vite URL map: absolute-ish path → hashed asset URL for each SVG. */
const iconUrlByPath = import.meta.glob('/node_modules/material-icon-theme/icons/*.svg', {
	eager: true,
	query: '?url',
	import: 'default'
}) as Record<string, string>;

const iconUrlByName = new Map<string, string>();
for (const [path, url] of Object.entries(iconUrlByPath)) {
	const base = path.split('/').pop() ?? '';
	if (!base.endsWith('.svg')) continue;
	iconUrlByName.set(base.slice(0, -'.svg'.length), url);
}

const DEFAULT_ICON = icons.file ?? 'file';

function basename(filePath: string): string {
	const parts = filePath.replace(/\\/g, '/').split('/');
	return parts[parts.length - 1] ?? filePath;
}

/**
 * Pick the Material icon key for a workspace-relative or absolute file path.
 * Matching order matches the VS Code theme: exact file name, then longest
 * compound extension (e.g. `test.ts` before `ts`).
 */
export function materialIconNameForPath(filePath: string, preferLight = true): string {
	const name = basename(filePath);
	const lower = name.toLowerCase();

	const lightNames = preferLight ? icons.light?.fileNames : undefined;
	const lightExts = preferLight ? icons.light?.fileExtensions : undefined;

	const byName = lightNames?.[lower] ?? icons.fileNames?.[lower];
	if (byName) return byName;

	const segments = lower.split('.');
	// "foo.test.ts" → try "test.ts", then "ts"
	if (segments.length > 1) {
		for (let i = 1; i < segments.length; i++) {
			const ext = segments.slice(i).join('.');
			const byExt = lightExts?.[ext] ?? icons.fileExtensions?.[ext];
			if (byExt) return byExt;
		}
	}

	return DEFAULT_ICON;
}

/** Resolved icon asset URL, or null when the SVG is missing from the package. */
export function materialIconUrlForPath(filePath: string, preferLight = true): string | null {
	const key = materialIconNameForPath(filePath, preferLight);
	return iconUrlByName.get(key) ?? iconUrlByName.get(DEFAULT_ICON) ?? null;
}

/** Available for tests / debugging — count of bundled SVG URLs. */
export function materialIconCatalogSize(): number {
	return iconUrlByName.size;
}
