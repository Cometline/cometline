import { tick } from 'svelte';
import { createMenuHighlight } from '$lib/features/composer/menu-highlight.svelte';
import { deleteWorkspace, forkSession, listWorkspaces } from '$lib/client/cometmind';
import type { ComposerInputRef } from '$lib/features/composer/composer-input-ref';
import {
	filterWorkspaceOptions,
	parseChangeCommand,
	type WorkspaceMenuOption
} from '$lib/features/skills/slash-commands';
import { gotoSession } from '$lib/routes/session-route';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

export function createComposerWorkspaceCommands(deps: {
	getValue: () => string;
	setValue: (value: string) => void;
	getInput: () => ComposerInputRef | null;
	getSessionId: () => string;
	onWorkspaceChanged?: () => void | Promise<void>;
	setDropMessage: (message: string) => void;
	focusInput: (options?: { position?: 'start' | 'end' }) => Promise<void>;
	getSkillMenuRef: () => HTMLDivElement | null;
	invalidateSkills: () => void;
}) {
	let workspacePaths = $state<string[]>([]);
	let workspaceSessionCounts = $state<Map<string, number>>(new Map());
	let workspacePathsLoading = $state(false);
	let workspacePathsLoaded = $state(false);
	let workspaceDeleting = $state(false);

	const changeCommand = $derived(parseChangeCommand(deps.getValue()));
	const workspaceMenuOpen = $derived(Boolean(changeCommand));
	const workspaceSearchQuery = $derived(changeCommand?.query ?? '');
	const filteredWorkspaceOptions = $derived.by(() => {
		if (!changeCommand) return [];
		return filterWorkspaceOptions(workspaceSearchQuery, workspacePaths, workspaceSessionCounts);
	});
	const highlight = createMenuHighlight({
		getQuery: () => workspaceSearchQuery,
		getOpen: () => workspaceMenuOpen,
		getCount: () => filteredWorkspaceOptions.length
	});

	$effect(() => {
		if (!workspaceMenuOpen) return;
		void ensureWorkspacePathsLoaded();
	});

	async function ensureWorkspacePathsLoaded() {
		if (workspacePathsLoaded || workspacePathsLoading) return;
		workspacePathsLoading = true;
		try {
			const recent = (await window.electronAPI?.listRecentWorkspaces?.()) ?? [];
			const registered = await listWorkspaces().catch(() => []);
			const counts = new Map<string, number>();
			for (const ws of registered) {
				counts.set(ws.path, ws.session_count);
			}
			workspaceSessionCounts = counts;
			const seen = new Set<string>();
			const merged: string[] = [];
			const add = (path: string) => {
				const clean = path.trim();
				if (!clean || seen.has(clean)) return;
				seen.add(clean);
				merged.push(clean);
			};
			for (const path of recent) add(path);
			add(shellStore.workspacePath);
			for (const ws of registered) add(ws.path);
			workspacePaths =
				(await window.electronAPI?.filterExistingWorkspacePaths?.(merged)) ?? merged;
			workspacePathsLoaded = true;
		} catch {
			workspacePaths = shellStore.workspacePath ? [shellStore.workspacePath] : [];
			workspacePathsLoaded = true;
		} finally {
			workspacePathsLoading = false;
		}
	}

	async function scrollHighlightedIntoView() {
		await tick();
		const option = deps
			.getSkillMenuRef()
			?.querySelector(`[data-workspace-index="${highlight.index}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
	}

	async function applyWorkspaceChange(path: string) {
		const clean = path.trim();
		if (!clean) return;
		try {
			let forkedId: string | null = null;
			const sessionId = deps.getSessionId();
			if (sessionId) {
				const forked = await forkSession(sessionId, clean);
				sessionStore.appendSession(forked);
				forkedId = forked.id;
			}
			shellStore.commitActiveWorkspace(clean);
			deps.invalidateSkills();
			workspacePathsLoaded = false;
			deps.getInput()?.clear();
			deps.setValue('');
			highlight.index = 0;
			if (forkedId) {
				// Remount-equivalent before soft navigate: empty fork must start
				// centered so first-turn flight + follow-up transitions work.
				shellStore.centerComposer();
				deps.setDropMessage(`Forked session into ${clean}`);
				await gotoSession(forkedId);
			} else {
				deps.setDropMessage(`Switched workspace to ${clean}`);
				await deps.onWorkspaceChanged?.();
			}
			void deps.focusInput();
		} catch (err) {
			deps.setDropMessage(err instanceof Error ? err.message : 'Failed to fork session');
		}
	}

	async function selectWorkspaceOption(option: WorkspaceMenuOption) {
		if (option.kind === 'browse') {
			const picked = await window.electronAPI?.browseWorkspacePath?.();
			if (!picked) return;
			await applyWorkspaceChange(picked);
			return;
		}
		await applyWorkspaceChange(option.path);
	}

	async function handleChangeWorkspaceSubmit(trimmed: string) {
		const parsed = parseChangeCommand(trimmed);
		if (!parsed) return;
		const option = filteredWorkspaceOptions[highlight.index];
		if (option?.kind === 'workspace') {
			await applyWorkspaceChange(option.path);
			return;
		}
		if (option?.kind === 'browse') {
			await selectWorkspaceOption(option);
			return;
		}
		if (parsed.query) {
			await applyWorkspaceChange(parsed.query);
		}
	}

	async function removeWorkspaceFromList(path: string, event: Event) {
		event.preventDefault();
		event.stopPropagation();
		if (workspaceDeleting) return;
		workspaceDeleting = true;
		try {
			await window.electronAPI?.removeRecentWorkspacePath?.(path);
			await deleteWorkspace(path);
			workspacePathsLoaded = false;
			await ensureWorkspacePathsLoaded();
			deps.setDropMessage(`Removed ${path} from workspace list`);
		} catch (err) {
			deps.setDropMessage(err instanceof Error ? err.message : 'Failed to remove workspace');
		} finally {
			workspaceDeleting = false;
		}
	}

	function handleKeydown(e: KeyboardEvent): boolean {
		if (!workspaceMenuOpen) return false;
		if (e.key === 'Escape') {
			e.preventDefault();
			deps.getInput()?.clear();
			deps.setValue('');
			return true;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (filteredWorkspaceOptions.length > 0) {
				highlight.index = (highlight.index + 1) % filteredWorkspaceOptions.length;
				void scrollHighlightedIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredWorkspaceOptions.length > 0) {
				highlight.index =
					(highlight.index - 1 + filteredWorkspaceOptions.length) %
					filteredWorkspaceOptions.length;
				void scrollHighlightedIntoView();
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const option = filteredWorkspaceOptions[highlight.index];
			if (!option) {
				if (e.key === 'Tab') {
					e.preventDefault();
					return true;
				}
				return false;
			}
			e.preventDefault();
			void selectWorkspaceOption(option);
			return true;
		}
		return false;
	}

	function prepareOpen() {
		highlight.index = 0;
		void ensureWorkspacePathsLoaded();
	}

	return {
		get workspaceMenuOpen() {
			return workspaceMenuOpen;
		},
		get workspaceSearchQuery() {
			return workspaceSearchQuery;
		},
		get workspacePathsLoading() {
			return workspacePathsLoading;
		},
		get workspacePathsLoaded() {
			return workspacePathsLoaded;
		},
		get filteredWorkspaceOptions() {
			return filteredWorkspaceOptions;
		},
		get workspaceHighlight() {
			return highlight.index;
		},
		set workspaceHighlight(index: number) {
			highlight.index = index;
		},
		get workspaceDeleting() {
			return workspaceDeleting;
		},
		selectWorkspaceOption,
		removeWorkspaceFromList,
		handleChangeWorkspaceSubmit,
		handleKeydown,
		prepareOpen
	};
}
