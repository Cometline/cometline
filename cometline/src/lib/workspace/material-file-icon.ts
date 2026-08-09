/** Resolve a curated Material Icon Theme subset without bundling its full manifest. */
import { MATERIAL_ICON_MAP } from './material-icon-map';

const icons = MATERIAL_ICON_MAP;

/** Vite URL map: absolute-ish path → hashed asset URL for each SVG. */
const iconUrlByPath = import.meta.glob(
	'/node_modules/material-icon-theme/icons/{file,nodejs,pnpm_light,yarn,readme,docker,makefile,license,git,vite,svelte,tsconfig,go-mod,python-misc,gemfile,tune,eslint,prettier,tailwindcss,typescript,react_ts,javascript,react,go,python,rust,css,sass,less,html,json,yaml,toml_light,markdown,mdx,document,console,database,xml,svg,image,pdf,table,log,lock,vue,java,kotlin,swift,c,h,cpp,hpp,csharp,php,ruby,lua,r,dart,elixir,erlang,fsharp,clojure,scala,proto,graphql,test-ts,test-jsx,test-js}.svg',
	{
		eager: true,
		query: '?url&no-inline',
		import: 'default'
	}
) as Record<string, string>;

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
export function materialIconNameForPath(filePath: string): string {
	const name = basename(filePath);
	const lower = name.toLowerCase();

	const byName = icons.fileNames[lower as keyof typeof icons.fileNames];
	if (byName) return byName;

	const segments = lower.split('.');
	// "foo.test.ts" → try "test.ts", then "ts"
	if (segments.length > 1) {
		for (let i = 1; i < segments.length; i++) {
			const ext = segments.slice(i).join('.');
			const byExt = icons.fileExtensions[ext as keyof typeof icons.fileExtensions];
			if (byExt) return byExt;
		}
	}

	return DEFAULT_ICON;
}

/** Resolved icon asset URL, or null when the SVG is missing from the package. */
export function materialIconUrlForPath(filePath: string): string | null {
	const key = materialIconNameForPath(filePath);
	return iconUrlByName.get(key) ?? iconUrlByName.get(DEFAULT_ICON) ?? null;
}

/** Available for tests / debugging — count of bundled SVG URLs. */
export function materialIconCatalogSize(): number {
	return iconUrlByName.size;
}
