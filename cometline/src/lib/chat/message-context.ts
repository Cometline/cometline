import type { MessageContextRef } from '$lib/generated/cometmind-api';
import type { WebContext } from '$lib/actions/start-chat';
import type { SelectionLineRange } from '$lib/workspace/selection-snippet';
import { openWorkspaceFilePreview } from '$lib/workspace/open-file-preview';
import { shellStore, type PendingWebContext } from '$lib/stores/shell.svelte';

export type { MessageContextRef };

/** Map wire web_contexts to slim UI refs (no content bodies). */
export function messageContextRefsFromWebContexts(
	webContexts: WebContext[] | undefined
): MessageContextRef[] | undefined {
	if (!webContexts?.length) return undefined;
	return webContexts.map((context) => {
		const ref: MessageContextRef = {
			kind: context.kind,
			source: context.source,
			...(context.title?.trim() ? { title: context.title.trim() } : {})
		};
		if (
			context.kind === 'file' &&
			!context.content.trim() &&
			!context.source.includes('#L')
		) {
			ref.role = 'viewing';
		}
		return ref;
	});
}

/** Map composer pending chips (incl. lazy page / viewing) to UI refs. */
export function messageContextRefsFromPending(
	pending: PendingWebContext[]
): MessageContextRef[] {
	return pending.map((context) => {
		const ref: MessageContextRef = {
			kind: context.kind,
			source: context.source,
			...(context.title?.trim() ? { title: context.title.trim() } : {})
		};
		if (context.kind === 'file' && 'role' in context && context.role === 'viewing') {
			ref.role = 'viewing';
		}
		return ref;
	});
}

export function messageContextLabel(context: MessageContextRef): string {
	if (context.kind === 'file' && context.role === 'viewing') {
		const name = context.title?.trim() || fileNameFromContextSource(context.source);
		return `Viewing ${name}`;
	}
	const title = context.title?.trim();
	if (title) return title;
	if (context.kind === 'terminal') return 'Terminal selection';
	if (context.kind === 'file') return fileNameFromContextSource(context.source);
	return pageNameFromContextSource(context.source);
}

export function fileNameFromContextSource(source: string): string {
	const { path } = parseFileContextSource(source);
	return path.split(/[/\\]/).filter(Boolean).pop() || path || 'File';
}

export function pageNameFromContextSource(source: string): string {
	try {
		const url = new URL(source);
		return url.hostname || source;
	} catch {
		return source || 'Page';
	}
}

export type ParsedFileContextSource = {
	path: string;
	range: SelectionLineRange | null;
};

/** Parse `workspace-file:path#L2-L3` / `@runtime/wiki/…#L2-L3` into path + line range. */
export function parseFileContextSource(source: string): ParsedFileContextSource {
	const trimmed = source.trim();
	const hashIndex = trimmed.indexOf('#');
	const base = hashIndex >= 0 ? trimmed.slice(0, hashIndex) : trimmed;
	const anchor = hashIndex >= 0 ? trimmed.slice(hashIndex + 1) : '';

	let path = base;
	if (path.startsWith('workspace-file:')) {
		path = path.slice('workspace-file:'.length);
	}

	const range = parseLineAnchor(anchor);
	return { path, range };
}

function parseLineAnchor(anchor: string): SelectionLineRange | null {
	const match = /^L(\d+)(?:-L?(\d+))?$/i.exec(anchor.trim());
	if (!match) return null;
	const startLine = Number(match[1]);
	const endLine = match[2] ? Number(match[2]) : startLine;
	if (!Number.isFinite(startLine) || !Number.isFinite(endLine) || startLine < 1 || endLine < 1) {
		return null;
	}
	return {
		startLine: Math.min(startLine, endLine),
		endLine: Math.max(startLine, endLine)
	};
}

/** Navigate from a context chip: file (+ optional lines), page URL, or terminal panel. */
export function openMessageContext(context: MessageContextRef): void {
	if (context.kind === 'page') {
		const url = context.source.trim();
		if (url) shellStore.openWorkspacePanelUrlForActive(url);
		return;
	}
	if (context.kind === 'terminal') {
		shellStore.openTerminalPanel();
		return;
	}
	if (context.kind === 'file') {
		const { path, range } = parseFileContextSource(context.source);
		if (!path) return;
		openWorkspaceFilePreview(path, range ?? undefined);
	}
}
