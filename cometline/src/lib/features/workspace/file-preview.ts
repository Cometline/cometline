const EXT_LANG: Record<string, string> = {
	'.ts': 'typescript',
	'.tsx': 'tsx',
	'.js': 'javascript',
	'.jsx': 'jsx',
	'.mjs': 'javascript',
	'.cjs': 'javascript',
	'.svelte': 'svelte',
	'.css': 'css',
	'.html': 'html',
	'.htm': 'html',
	'.json': 'json',
	'.md': 'markdown',
	'.markdown': 'markdown',
	'.yaml': 'yaml',
	'.yml': 'yaml',
	'.go': 'go',
	'.py': 'python',
	'.rs': 'rust',
	'.sql': 'sql',
	'.sh': 'bash',
	'.bash': 'bash',
	'.zsh': 'bash',
	'.toml': 'yaml',
	'.diff': 'diff'
};

const IMAGE_EXTENSIONS = new Set(['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg']);
const MARKDOWN_EXTENSIONS = new Set(['.md', '.markdown']);

export function extensionFromPath(filePath: string): string {
	const slash = Math.max(filePath.lastIndexOf('/'), filePath.lastIndexOf('\\'));
	const name = slash >= 0 ? filePath.slice(slash) : filePath;
	const dot = name.lastIndexOf('.');
	return dot >= 0 ? name.slice(dot).toLowerCase() : '';
}

export function languageFromPath(filePath: string): string | null {
	const ext = extensionFromPath(filePath);
	return languageFromExtension(ext);
}

export function languageFromExtension(extension: string): string | null {
	if (!extension) return null;
	const ext = extension.startsWith('.') ? extension.toLowerCase() : `.${extension.toLowerCase()}`;
	return EXT_LANG[ext] ?? null;
}

export function isMarkdownPath(filePath: string): boolean {
	return MARKDOWN_EXTENSIONS.has(extensionFromPath(filePath));
}

export function isImagePath(filePath: string): boolean {
	return IMAGE_EXTENSIONS.has(extensionFromPath(filePath));
}

export function isPdfPath(filePath: string): boolean {
	return extensionFromPath(filePath) === '.pdf';
}

/** Skip a watch/focus reload when the on-disk text matches the open buffer. */
export function shouldSkipTextPreviewReload(
	keepView: boolean,
	previewKind: 'text' | 'image' | 'pdf' | null,
	savedContent: string,
	incomingContent: string
): boolean {
	return keepView && previewKind === 'text' && savedContent === incomingContent;
}
