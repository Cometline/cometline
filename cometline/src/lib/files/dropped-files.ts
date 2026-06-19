export const MAX_DROPPED_FILES = 8;
export const MAX_DROPPED_FILE_BYTES = 256 * 1024;

export interface DroppedFileLike {
	name: string;
	size: number;
	type: string;
	text(): Promise<string>;
}

export interface AcceptedDroppedFile {
	name: string;
	language: string;
	text: string;
}

export interface RejectedDroppedFile {
	name: string;
	reason: string;
}

export interface DroppedFileReadResult {
	accepted: AcceptedDroppedFile[];
	rejected: RejectedDroppedFile[];
}

const TEXT_EXTENSIONS = new Set([
	'bash',
	'c',
	'cc',
	'cpp',
	'css',
	'csv',
	'go',
	'h',
	'html',
	'java',
	'js',
	'json',
	'jsx',
	'log',
	'md',
	'mjs',
	'py',
	'rs',
	'sh',
	'svelte',
	'toml',
	'ts',
	'tsx',
	'txt',
	'xml',
	'yaml',
	'yml'
]);

const TEXT_MIME_TYPES = new Set([
	'application/javascript',
	'application/json',
	'application/typescript',
	'application/xml',
	'application/x-sh',
	'application/x-yaml'
]);

const LANGUAGE_BY_EXTENSION: Record<string, string> = {
	bash: 'bash',
	c: 'c',
	cc: 'cpp',
	cpp: 'cpp',
	css: 'css',
	csv: 'csv',
	go: 'go',
	h: 'c',
	html: 'html',
	java: 'java',
	js: 'javascript',
	json: 'json',
	jsx: 'jsx',
	log: 'log',
	md: 'markdown',
	mjs: 'javascript',
	py: 'python',
	rs: 'rust',
	sh: 'bash',
	svelte: 'svelte',
	toml: 'toml',
	ts: 'typescript',
	tsx: 'tsx',
	txt: 'text',
	xml: 'xml',
	yaml: 'yaml',
	yml: 'yaml'
};

function extensionFor(name: string): string {
	const index = name.lastIndexOf('.');
	if (index < 0 || index === name.length - 1) return '';
	return name.slice(index + 1).toLowerCase();
}

export function isSupportedTextFile(file: DroppedFileLike): boolean {
	const type = file.type.toLowerCase();
	if (type.startsWith('text/')) return true;
	if (TEXT_MIME_TYPES.has(type)) return true;
	return TEXT_EXTENSIONS.has(extensionFor(file.name));
}

export function languageForFilename(name: string): string {
	return LANGUAGE_BY_EXTENSION[extensionFor(name)] ?? '';
}

function fenceFor(text: string): string {
	const runs = text.match(/`{3,}/g) ?? [];
	const longest = runs.reduce((max, run) => Math.max(max, run.length), 2);
	return '`'.repeat(longest + 1);
}

export function formatDroppedFiles(files: AcceptedDroppedFile[]): string {
	return files
		.map((file) => {
			const fence = fenceFor(file.text);
			const language = file.language ? file.language : '';
			return `[File: ${file.name}]\n${fence}${language}\n${file.text.trimEnd()}\n${fence}`;
		})
		.join('\n\n');
}

function kilobyteLabel(bytes: number): string {
	return `${Math.max(1, Math.floor(bytes / 1024))} KB`;
}

export async function readDroppedTextFiles(
	files: DroppedFileLike[],
	options: { maxFiles?: number; maxBytes?: number } = {}
): Promise<DroppedFileReadResult> {
	const maxFiles = options.maxFiles ?? MAX_DROPPED_FILES;
	const maxBytes = options.maxBytes ?? MAX_DROPPED_FILE_BYTES;
	const accepted: AcceptedDroppedFile[] = [];
	const rejected: RejectedDroppedFile[] = [];

	for (let index = 0; index < files.length; index += 1) {
		const file = files[index];
		if (index >= maxFiles) {
			rejected.push({ name: file.name, reason: `Only ${maxFiles} files can be dropped at once.` });
			continue;
		}
		if (!isSupportedTextFile(file)) {
			rejected.push({ name: file.name, reason: 'Unsupported file type.' });
			continue;
		}
		if (file.size > maxBytes) {
			rejected.push({ name: file.name, reason: `File is larger than ${kilobyteLabel(maxBytes)}.` });
			continue;
		}

		accepted.push({
			name: file.name,
			language: languageForFilename(file.name),
			text: await file.text()
		});
	}

	return { accepted, rejected };
}
