import { getActiveSessionId } from '#lib/active-session.js';
import type { WebContext } from '#lib/actions/start-chat.js';

/** A page selected for the next turn whose body has not been read yet. */
export type PendingPageContext = {
	kind: 'page';
	title?: string;
	source: string;
	lazy: true;
};

/** Path-only “currently viewing” file reference (no body attached). */
export type PendingViewingFileContext = WebContext & {
	kind: 'file';
	role: 'viewing';
	content: '';
};

export type PendingWebContext = WebContext | PendingPageContext | PendingViewingFileContext;

function isPendingPageContext(context: PendingWebContext): context is PendingPageContext {
	return context.kind === 'page' && 'lazy' in context && context.lazy;
}

function isViewingFileContext(context: PendingWebContext): context is PendingViewingFileContext {
	return context.kind === 'file' && 'role' in context && context.role === 'viewing';
}

function toWireWebContext(context: PendingWebContext): WebContext | null {
	if (isPendingPageContext(context)) return null;
	if (isViewingFileContext(context)) {
		return {
			kind: 'file',
			title: context.title,
			source: context.source,
			content: ''
		};
	}
	return {
		kind: context.kind,
		title: context.title,
		source: context.source,
		content: context.content
	};
}

/** Last visible page/file key. Not $state — noting context must not retrigger the caller. */
let lastNotedVisibleContextKey = '';

/** Per-session pending web/page/file context attached to the next composer turn. */
export function createWebContextStore() {
	let webContextsBySession = $state<Record<string, PendingWebContext[]>>({});
	let resolvePageContext: ((source: string) => Promise<WebContext | null>) | null = null;

	function clearForSession(sessionId: string) {
		if (!(sessionId in webContextsBySession)) return;
		const nextContexts = { ...webContextsBySession };
		delete nextContexts[sessionId];
		webContextsBySession = nextContexts;
	}

	function setViewingFileContextForActive(source: string, title: string) {
		const key = getActiveSessionId();
		if (!key) return;
		const current = webContextsBySession[key] ?? [];
		const existingViewing = current.find(isViewingFileContext);
		if (existingViewing?.source === source && (existingViewing.title ?? '') === title) {
			return;
		}
		const existing = current.filter((item) => !isViewingFileContext(item));
		const viewing: PendingViewingFileContext = {
			kind: 'file',
			role: 'viewing',
			title,
			source,
			content: ''
		};
		webContextsBySession = {
			...webContextsBySession,
			[key]: [...existing, viewing]
		};
	}

	function setPendingPageContextForActive(context: Omit<PendingPageContext, 'kind' | 'lazy'>) {
		const key = getActiveSessionId();
		if (!key) return;
		const existing = (webContextsBySession[key] ?? []).filter(
			(item) => !isPendingPageContext(item)
		);
		webContextsBySession = {
			...webContextsBySession,
			[key]: [
				...existing,
				{ kind: 'page', title: context.title, source: context.source, lazy: true }
			]
		};
	}

	return {
		get pendingWebContexts(): PendingWebContext[] {
			const key = getActiveSessionId();
			return key ? (webContextsBySession[key] ?? []) : [];
		},
		clearForSession,
		addWebContextForActive(context: WebContext) {
			const key = getActiveSessionId();
			if (!key) return;
			const existing = webContextsBySession[key] ?? [];
			const nextContext: WebContext = {
				...context,
				content: context.content.trim().slice(0, 50000)
			};
			if (
				nextContext.kind === 'message' &&
				existing.some(
					(item) =>
						item.kind === 'message' &&
						item.source === nextContext.source &&
						item.content.replace(/\s+/g, ' ').trim() ===
							nextContext.content.replace(/\s+/g, ' ').trim()
				)
			) {
				return;
			}
			let next = existing;
			if (context.kind === 'page') {
				next = existing.filter(
					(item) => !(isPendingPageContext(item) && item.source === context.source)
				);
			}
			webContextsBySession = {
				...webContextsBySession,
				[key]: [...next, nextContext]
			};
		},
		setViewingFileContextForActive,
		noteVisibleContext(visible: {
			page?: { source: string; title: string };
			file?: { source: string; title: string };
		}) {
			const key = getActiveSessionId();
			if (!key) return;
			const pageKey = visible.page
				? `page:${visible.page.source}\0${visible.page.title}`
				: '';
			const fileKey = visible.file
				? `file:${visible.file.source}\0${visible.file.title}`
				: '';
			const nextKey = `${key}|${pageKey}|${fileKey}`;
			if (nextKey === lastNotedVisibleContextKey) return;
			lastNotedVisibleContextKey = nextKey;
			if (visible.page) setPendingPageContextForActive(visible.page);
			if (visible.file)
				setViewingFileContextForActive(visible.file.source, visible.file.title);
		},
		setPendingPageContextForActive,
		registerPageContextResolver(resolver: (source: string) => Promise<WebContext | null>) {
			resolvePageContext = resolver;
			return () => {
				if (resolvePageContext === resolver) resolvePageContext = null;
			};
		},
		async resolvePendingWebContextsForActive(): Promise<WebContext[]> {
			const key = getActiveSessionId();
			const contexts = key ? [...(webContextsBySession[key] ?? [])] : [];
			const resolved = await Promise.all(
				contexts.map(async (context) => {
					if (isPendingPageContext(context)) {
						return resolvePageContext?.(context.source) ?? null;
					}
					return toWireWebContext(context);
				})
			);
			return resolved.filter((context): context is WebContext => context !== null);
		},
		removeWebContextAt(index: number) {
			const key = getActiveSessionId();
			if (!key) return;
			const existing = webContextsBySession[key] ?? [];
			if (index < 0 || index >= existing.length) return;
			const next = existing.filter((_, i) => i !== index);
			if (next.length === 0) {
				const copy = { ...webContextsBySession };
				delete copy[key];
				webContextsBySession = copy;
				return;
			}
			webContextsBySession = {
				...webContextsBySession,
				[key]: next
			};
		},
		clearWebContextForActive() {
			const key = getActiveSessionId();
			if (!key || !(key in webContextsBySession)) return;
			const next = { ...webContextsBySession };
			delete next[key];
			webContextsBySession = next;
		}
	};
}

export type WebContextStore = ReturnType<typeof createWebContextStore>;
