<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { fade, fly } from 'svelte/transition';
	import {
		Check,
		ChevronDown,
		FileText,
		Folder,
		Loader,
		Search,
		Send,
		Sparkles,
		Square,
		X
	} from '@lucide/svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';
	import RichComposerInput from '$lib/components/RichComposerInput.svelte';
	import { listSkills, listWorkspaces, forkSession } from '$lib/client/cometmind';
	import {
		filterFileIndex,
		getFileIndex,
		isFileIndexFresh,
		isFileIndexReady,
		refreshFileIndex,
		searchWorkspaceFiles
	} from '$lib/workspace/file-index';
	import { sessionStore } from '$lib/stores/session.svelte';
	import {
		BUILTIN_SLASH_COMMANDS,
		expandBuiltinSlashCommand,
		filterSlashMenuOptions,
		filterWorkspaceOptions,
		isChangeWorkspaceCommand,
		parseChangeCommand,
		parseModelCommand,
		type SlashMenuOption,
		type WorkspaceMenuOption
	} from '$lib/skills/slash-commands';
	import { formatDroppedFiles, readDroppedTextFiles } from '$lib/files/dropped-files';
	import { workspaceLabel } from '$lib/sessions/group-by-workspace';
	import { imageDataURL, isSupportedImageFile, readImageAttachments } from '$lib/files/images';
	import type { ImageAttachment, SkillResource } from '$lib/types';

	let {
		onSend,
		onStop,
		onRemoveQueued,
		onModelChange,
		onWorkspaceChanged,
		sessionId = '',
		disabled = false,
		streaming = false,
		queuedCount = 0,
		queuedMessages = [],
		waitingForReply = false,
		variant = 'dock',
		autofocus = true
	}: {
		onSend: (text: string, images?: ImageAttachment[], filePaths?: string[]) => void;
		onStop?: () => void;
		onRemoveQueued?: (id: string) => void;
		onModelChange?: (option: ModelOption) => void | Promise<void>;
		onWorkspaceChanged?: () => void | Promise<void>;
		sessionId?: string;
		disabled?: boolean;
		streaming?: boolean;
		queuedCount?: number;
		queuedMessages?: QueuedMessage[];
		waitingForReply?: boolean;
		variant?: 'hero' | 'dock';
		autofocus?: boolean;
	} = $props();

	let value = $state('');
	let images = $state<ImageAttachment[]>([]);
	let input = $state<RichComposerInput | null>(null);
	let modelOpen = $state(false);
	let modelSearch = $state('');
	let modelSearchInput = $state<HTMLInputElement | null>(null);
	let queuePreviewOpen = $state(false);
	let queuePicker = $state<HTMLDivElement | null>(null);
	let skillMenu = $state<HTMLDivElement | null>(null);
	let skills = $state<SkillResource[]>([]);
	let skillsLoaded = $state(false);
	let skillsLoading = $state(false);
	let skillHighlight = $state(0);
	let workspaceHighlight = $state(0);
	let workspacePaths = $state<string[]>([]);
	let workspacePathsLoading = $state(false);
	let workspacePathsLoaded = $state(false);
	let dismissedSkillCommand = $state('');
	let mentionMenu = $state<HTMLDivElement | null>(null);
	let mentionQuery = $state('');
	let mentionMenuOpen = $state(false);
	let mentionHighlight = $state(0);
	// The file-index cache lives outside Svelte's reactivity (a plain module
	// Map), so bump this version after a refresh to recompute the derived state.
	let mentionIndexVersion = $state(0);
	// Server-side search results, used only when the workspace index is
	// truncated (more files than the cached page) and the user is typing.
	let mentionServerResults = $state<string[]>([]);
	let mentionServerQuery = $state('');
	let mentionServerLoading = $state(false);
	let mentionSearchTimer: ReturnType<typeof setTimeout> | null = null;
	let mentionSearchSeq = 0;
	let dragDepth = $state(0);
	let dropMessage = $state('');
	let dropProcessing = $state(false);
	let dropMessageTimer: ReturnType<typeof setTimeout> | null = null;
	let dragActive = $derived(dragDepth > 0 || dropProcessing);
	let canSubmit = $derived(Boolean(value.trim() || images.length > 0));
	let skillCommandMatch = $derived(/^\s*\/([\w-]*)$/.exec(value));
	let skillCommandQuery = $derived(skillCommandMatch?.[1]?.toLowerCase() ?? '');
	let skillMenuOpen = $derived(
		Boolean(skillCommandMatch && skillCommandMatch[0] !== dismissedSkillCommand)
	);
	let filteredSlashOptions = $derived.by(() => {
		if (!skillCommandMatch) return [];
		return filterSlashMenuOptions(skillCommandQuery, skills);
	});
	let changeCommand = $derived(parseChangeCommand(value));
	let workspaceMenuOpen = $derived(Boolean(changeCommand));
	let workspaceSearchQuery = $derived(changeCommand?.query ?? '');
	let filteredWorkspaceOptions = $derived.by(() => {
		if (!changeCommand) return [];
		return filterWorkspaceOptions(workspaceSearchQuery, workspacePaths);
	});
	let modelCommand = $derived(parseModelCommand(value));
	let modelCommandMenuOpen = $derived(Boolean(modelCommand));
	let modelCommandQuery = $derived(modelCommand?.query ?? '');
	let modelCommandHighlight = $state(0);
	let filteredModelCommandOptions = $derived.by(() => {
		const query = modelCommandQuery.trim().toLowerCase();
		if (!query) return modelStore.options;
		return modelStore.options.filter(
			(option) =>
				option.label.toLowerCase().includes(query) ||
				option.modelId.toLowerCase().includes(query) ||
				option.providerName.toLowerCase().includes(query)
		);
	});
	let groupedModelCommandOptions = $derived.by(() => {
		const groups: {
			providerId: string;
			providerName: string;
			providerMethod: string;
			options: ModelOption[];
		}[] = [];
		for (const option of filteredModelCommandOptions) {
			let group = groups.find((item) => item.providerId === option.providerId);
			if (!group) {
				group = {
					providerId: option.providerId,
					providerName: option.providerName,
					providerMethod: option.providerMethod,
					options: []
				};
				groups.push(group);
			}
			group.options.push(option);
		}
		return groups;
	});
	let skillNames = $derived([
		...BUILTIN_SLASH_COMMANDS.map((cmd) => cmd.name),
		...skills.map((skill) => skill.name)
	]);
	let fileIndex = $derived.by(() => {
		void mentionIndexVersion;
		return getFileIndex(shellStore.workspacePath);
	});
	// Mentions are available whenever there is a workspace to index. The picker
	// itself shows an "indexing" state while the file list is still loading.
	let hasWorkspace = $derived(
		Boolean(shellStore.workspacePath) && shellStore.workspacePath !== '/'
	);
	let currentWorkspaceLabel = $derived(
		hasWorkspace ? workspaceLabel(shellStore.workspacePath) : ''
	);
	let fileIndexReady = $derived.by(() => {
		void mentionIndexVersion;
		return isFileIndexReady(shellStore.workspacePath);
	});
	let mentionTruncated = $derived(Boolean(fileIndex?.truncated));
	// When the index is complete, filter the cached list locally (instant). When
	// it is truncated and the user has typed, defer to server-side search so
	// files outside the cached page are still findable.
	let useServerSearch = $derived(mentionTruncated && mentionQuery.trim().length > 0);
	let filteredMentionFiles = $derived.by(() => {
		if (useServerSearch) {
			// Only trust server results that match the current query.
			if (mentionServerQuery === mentionQuery.trim()) return mentionServerResults;
			return [];
		}
		const files = fileIndex?.files ?? [];
		return filterFileIndex(files, mentionQuery);
	});
	let filteredModelOptions = $derived.by(() => {
		const query = modelSearch.trim().toLowerCase();
		if (!query) return modelStore.options;
		return modelStore.options.filter(
			(option) =>
				option.label.toLowerCase().includes(query) ||
				option.modelId.toLowerCase().includes(query) ||
				option.providerName.toLowerCase().includes(query)
		);
	});
	let groupedModelOptions = $derived.by(() => {
		const groups: {
			providerId: string;
			providerName: string;
			providerMethod: string;
			options: ModelOption[];
		}[] = [];
		for (const option of filteredModelOptions) {
			let group = groups.find((item) => item.providerId === option.providerId);
			if (!group) {
				group = {
					providerId: option.providerId,
					providerName: option.providerName,
					providerMethod: option.providerMethod,
					options: []
				};
				groups.push(group);
			}
			group.options.push(option);
		}
		return groups;
	});

	export function focus() {
		void focusInput();
	}

	$effect(() => {
		if (!autofocus) return;
		void sessionId;
		void focusInput();
	});

	$effect(() => {
		const workspacePath = shellStore.workspacePath;
		if (!workspacePath) return;
		if (isFileIndexReady(workspacePath)) return;
		// Defer the (potentially expensive) workspace file walk until the
		// browser is idle, so it never competes with the transcript/session
		// loads that fire on the same session switch. The walk is only needed
		// for the @-mention picker, which the user reaches later.
		const handle = scheduleIdle(() => {
			if (shellStore.workspacePath === workspacePath && !isFileIndexReady(workspacePath)) {
				void loadMentionIndex(workspacePath);
			}
		});
		return () => cancelIdle(handle);
	});

	$effect(() => {
		if (queuedCount === 0) queuePreviewOpen = false;
	});

	$effect(() => {
		if (!skillCommandMatch) {
			dismissedSkillCommand = '';
			return;
		}
		void ensureSkillsLoaded();
	});

	$effect(() => {
		if (!skillMenuOpen) return;
		if (skillHighlight >= filteredSlashOptions.length) {
			skillHighlight = Math.max(0, filteredSlashOptions.length - 1);
		}
	});

	$effect(() => {
		if (!workspaceMenuOpen) return;
		void ensureWorkspacePathsLoaded();
		if (workspaceHighlight >= filteredWorkspaceOptions.length) {
			workspaceHighlight = Math.max(0, filteredWorkspaceOptions.length - 1);
		}
	});

	$effect(() => {
		if (!mentionMenuOpen) return;
		if (mentionHighlight >= filteredMentionFiles.length) {
			mentionHighlight = Math.max(0, filteredMentionFiles.length - 1);
		}
	});

	// Debounced server-side search for truncated workspaces. Only runs while the
	// picker is open, the index is truncated, and the user has typed a query.
	$effect(() => {
		const workspacePath = shellStore.workspacePath;
		const query = mentionQuery.trim();
		if (!mentionMenuOpen || !useServerSearch) {
			if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
			mentionSearchTimer = null;
			return;
		}
		if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
		const seq = ++mentionSearchSeq;
		mentionServerLoading = true;
		mentionSearchTimer = setTimeout(() => {
			void searchWorkspaceFiles(workspacePath, query)
				.then((files) => {
					if (seq !== mentionSearchSeq) return; // stale
					mentionServerResults = files;
					mentionServerQuery = query;
				})
				.catch(() => {
					if (seq !== mentionSearchSeq) return;
					mentionServerResults = [];
					mentionServerQuery = query;
				})
				.finally(() => {
					if (seq === mentionSearchSeq) mentionServerLoading = false;
				});
		}, 150);
		return () => {
			if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
			mentionSearchTimer = null;
		};
	});

	$effect(() => {
		if (!queuePreviewOpen) return;
		function onPointerDown(e: PointerEvent) {
			if (queuePicker?.contains(e.target as Node)) return;
			queuePreviewOpen = false;
		}
		document.addEventListener('pointerdown', onPointerDown);
		return () => document.removeEventListener('pointerdown', onPointerDown);
	});

	onDestroy(() => {
		if (dropMessageTimer) clearTimeout(dropMessageTimer);
	});

	function submit() {
		const trimmed = value.trim();
		if (isChangeWorkspaceCommand(trimmed)) {
			void handleChangeWorkspaceSubmit(trimmed);
			return;
		}
		if (modelCommand) {
			void handleModelCommandSubmit();
			return;
		}
		const expanded = expandBuiltinSlashCommand(trimmed) ?? expandSkillCommand(trimmed);
		if (!canSubmit || disabled || !modelStore.selected) return;
		const filePaths = input?.getFilePaths() ?? [];
		onSend(
			expanded,
			images.length > 0 ? images : undefined,
			filePaths.length > 0 ? filePaths : undefined
		);
		input?.clear();
		value = '';
		images = [];
	}

	function onKeydown(e: KeyboardEvent) {
		if (handleWorkspaceMenuKeydown(e)) return;
		if (handleModelCommandMenuKeydown(e)) return;
		if (handleSkillMenuKeydown(e)) return;
		if (handleMentionMenuKeydown(e)) return;
		if (matchesShortcut(e, settingsStore.settings.shortcuts.stopResponse) && streaming) {
			// Only intercept when there's no text selection in the editor.
			const sel = window.getSelection();
			if (!sel || sel.isCollapsed) {
				e.preventDefault();
				onStop?.();
				return;
			}
		}
		if (!e.isComposing && matchesShortcut(e, settingsStore.settings.shortcuts.insertNewline)) {
			e.preventDefault();
			input?.insertText('\n');
			return;
		}
		if (!e.isComposing && matchesShortcut(e, settingsStore.settings.shortcuts.sendMessage)) {
			e.preventDefault();
			submit();
		}
	}

	async function ensureSkillsLoaded() {
		if (skillsLoaded || skillsLoading) return;
		skillsLoading = true;
		try {
			const result = await listSkills(shellStore.workspacePath);
			skills = result.skills.filter((skill) => !skill.internal);
			skillsLoaded = true;
		} catch {
			skills = [];
			skillsLoaded = true;
		} finally {
			skillsLoading = false;
		}
	}

	async function ensureWorkspacePathsLoaded() {
		if (workspacePathsLoaded || workspacePathsLoading) return;
		workspacePathsLoading = true;
		try {
			const recent = (await window.electronAPI?.listRecentWorkspaces?.()) ?? [];
			const registered = await listWorkspaces().catch(() => []);
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

	function handleWorkspaceMenuKeydown(e: KeyboardEvent): boolean {
		if (!workspaceMenuOpen) return false;
		if (e.key === 'Escape') {
			e.preventDefault();
			input?.clear();
			value = '';
			return true;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (filteredWorkspaceOptions.length > 0) {
				workspaceHighlight = (workspaceHighlight + 1) % filteredWorkspaceOptions.length;
				void scrollHighlightedWorkspaceIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredWorkspaceOptions.length > 0) {
				workspaceHighlight =
					(workspaceHighlight - 1 + filteredWorkspaceOptions.length) %
					filteredWorkspaceOptions.length;
				void scrollHighlightedWorkspaceIntoView();
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const option = filteredWorkspaceOptions[workspaceHighlight];
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

	function handleModelCommandMenuKeydown(e: KeyboardEvent): boolean {
		if (!modelCommandMenuOpen) return false;
		const flatOptions = filteredModelCommandOptions;
		if (e.key === 'Escape') {
			e.preventDefault();
			input?.clear();
			value = '';
			return true;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (flatOptions.length > 0) {
				modelCommandHighlight = (modelCommandHighlight + 1) % flatOptions.length;
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (flatOptions.length > 0) {
				modelCommandHighlight =
					(modelCommandHighlight - 1 + flatOptions.length) % flatOptions.length;
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const option = flatOptions[modelCommandHighlight];
			if (!option) {
				if (e.key === 'Tab') {
					e.preventDefault();
					return true;
				}
				return false;
			}
			e.preventDefault();
			void selectModelCommandOption(option);
			return true;
		}
		return false;
	}

	async function scrollHighlightedWorkspaceIntoView() {
		await tick();
		const option = skillMenu?.querySelector(`[data-workspace-index="${workspaceHighlight}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
	}

	async function selectWorkspaceOption(option: WorkspaceMenuOption) {
		if (option.kind === 'browse') {
			const picked = await window.electronAPI?.selectWorkspacePath?.();
			if (!picked) return;
			await applyWorkspaceChange(picked);
			return;
		}
		await applyWorkspaceChange(option.path);
	}

	async function handleChangeWorkspaceSubmit(trimmed: string) {
		const parsed = parseChangeCommand(trimmed);
		if (!parsed) return;
		const option = filteredWorkspaceOptions[workspaceHighlight];
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

	async function applyWorkspaceChange(path: string) {
		const clean = path.trim();
		if (!clean) return;
		try {
			let forkedId: string | null = null;
			if (sessionId) {
				const forked = await forkSession(sessionId, clean);
				sessionStore.appendSession(forked);
				forkedId = forked.id;
			}
			await window.electronAPI?.setWorkspacePath?.(clean);
			shellStore.commitWorkspace(clean);
			skillsLoaded = false;
			skills = [];
			workspacePathsLoaded = false;
			input?.clear();
			value = '';
			workspaceHighlight = 0;
			if (forkedId) {
				setDropMessage(`Forked session into ${clean}`);
				await goto(`/session/${forkedId}`);
			} else {
				setDropMessage(`Switched workspace to ${clean}`);
				await onWorkspaceChanged?.();
			}
			void focusInput();
		} catch (err) {
			setDropMessage(err instanceof Error ? err.message : 'Failed to fork session');
		}
	}

	async function selectModelCommandOption(option: ModelOption) {
		modelStore.select(option);
		await onModelChange?.(option);
		input?.clear();
		value = '';
		modelCommandHighlight = 0;
		setDropMessage(`Switched to ${option.label}`);
	}

	function handleModelCommandSubmit() {
		const flatOptions = filteredModelCommandOptions;
		const option = flatOptions[modelCommandHighlight];
		if (option) {
			void selectModelCommandOption(option);
			return;
		}
		input?.clear();
		value = '';
	}

	function handleSkillMenuKeydown(e: KeyboardEvent): boolean {
		if (!skillMenuOpen) return false;
		if (e.key === 'Escape') {
			e.preventDefault();
			dismissedSkillCommand = skillCommandMatch?.[0] ?? value;
			return true;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (filteredSlashOptions.length > 0) {
				skillHighlight = (skillHighlight + 1) % filteredSlashOptions.length;
				void scrollHighlightedSkillIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredSlashOptions.length > 0) {
				skillHighlight =
					(skillHighlight - 1 + filteredSlashOptions.length) %
					filteredSlashOptions.length;
				void scrollHighlightedSkillIntoView();
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const option = filteredSlashOptions[skillHighlight];
			if (!option) {
				if (e.key === 'Tab') {
					e.preventDefault();
					return true;
				}
				return false;
			}
			e.preventDefault();
			selectSlashOption(option);
			return true;
		}
		return false;
	}

	function handleMentionMenuKeydown(e: KeyboardEvent): boolean {
		if (!mentionMenuOpen) return false;
		if (e.key === 'Escape') {
			e.preventDefault();
			closeMentionMenu();
			return true;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (filteredMentionFiles.length > 0) {
				mentionHighlight = (mentionHighlight + 1) % filteredMentionFiles.length;
				void scrollHighlightedMentionIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredMentionFiles.length > 0) {
				mentionHighlight =
					(mentionHighlight - 1 + filteredMentionFiles.length) %
					filteredMentionFiles.length;
				void scrollHighlightedMentionIntoView();
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const path = filteredMentionFiles[mentionHighlight];
			if (!path) {
				if (e.key === 'Tab') {
					e.preventDefault();
					return true;
				}
				return false;
			}
			e.preventDefault();
			selectMentionFile(path);
			return true;
		}
		return false;
	}

	function closeMentionMenu() {
		mentionMenuOpen = false;
		mentionQuery = '';
		mentionServerResults = [];
		mentionServerQuery = '';
		mentionServerLoading = false;
		mentionSearchSeq += 1;
		if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
		mentionSearchTimer = null;
	}

	async function scrollHighlightedMentionIntoView() {
		await tick();
		const option = mentionMenu?.querySelector(`[data-mention-index="${mentionHighlight}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
	}

	function selectMentionFile(path: string) {
		input?.insertFileMention(path);
		closeMentionMenu();
	}

	function onMentionQuery(payload: { query: string; active: boolean }) {
		if (!payload.active) {
			closeMentionMenu();
			return;
		}
		if (!hasWorkspace) return;
		// Open the picker even while indexing; it shows a loading state and
		// fills in automatically once the file index is ready. If the cached
		// index is stale (TTL elapsed), refresh in the background — the cached
		// list keeps the picker instant while fresh files load in.
		if (!isFileIndexFresh(shellStore.workspacePath)) {
			void loadMentionIndex(shellStore.workspacePath);
		}
		mentionQuery = payload.query;
		mentionMenuOpen = true;
		mentionHighlight = 0;
	}

	async function loadMentionIndex(workspacePath: string) {
		try {
			await refreshFileIndex(workspacePath);
		} finally {
			// Recompute fileIndex/fileIndexReady now that the cache changed.
			mentionIndexVersion += 1;
		}
	}

	async function scrollHighlightedSkillIntoView() {
		await tick();
		const option = skillMenu?.querySelector(`[data-skill-index="${skillHighlight}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
	}

	function openChangeWorkspace() {
		const next = '/change ';
		input?.setText(next);
		value = next;
		dismissedSkillCommand = '';
		skillHighlight = 0;
		workspaceHighlight = 0;
		void ensureWorkspacePathsLoaded();
		void focusInput();
	}

	function selectSlashOption(option: SlashMenuOption) {
		if (option.kind === 'builtin' && option.name === 'change') {
			openChangeWorkspace();
			return;
		}
		const next = `/${option.name} `;
		input?.setText(next);
		value = next;
		dismissedSkillCommand = next;
		skillHighlight = 0;
	}

	function parseLeadingSkillCommand(text: string) {
		const match = /^\s*\/([\w-]+)(?:\s+([\s\S]*))?$/.exec(text);
		if (!match) return null;
		const skillName = match[1];
		if (!skills.some((skill) => skill.name === skillName)) return null;
		return { skillName, rest: match[2]?.trimStart() ?? '' };
	}

	function expandSkillCommand(text: string) {
		const command = parseLeadingSkillCommand(text);
		if (!command) return text;
		const rest = command.rest ? `\n\n${command.rest}` : '';
		return `Use the \`${command.skillName}\` skill for this request. Load it with the \`load_skill\` tool before proceeding.${rest}`;
	}

	function selectModel(option: ModelOption) {
		modelStore.select(option);
		void onModelChange?.(option);
		modelOpen = false;
		modelSearch = '';
	}

	async function openModelMenu() {
		if (modelStore.options.length === 0) return;
		modelOpen = true;
		modelSearch = '';
		await tick();
		modelSearchInput?.focus();
		modelSearchInput?.select();
	}

	function toggleModelMenu() {
		if (modelOpen) {
			modelOpen = false;
			modelSearch = '';
			return;
		}
		void openModelMenu();
	}

	function closeModelMenu(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		const current = e.currentTarget as Node;
		if (next && current.contains(next)) return;
		modelOpen = false;
		modelSearch = '';
	}

	function toggleQueuePreview() {
		queuePreviewOpen = !queuePreviewOpen;
	}

	function removeQueued(id: string) {
		onRemoveQueued?.(id);
	}

	function hasDroppedFiles(dataTransfer: DataTransfer | null): boolean {
		return dataTransfer?.types.includes('Files') ?? false;
	}

	function setDropMessage(message: string) {
		dropMessage = message;
		if (dropMessageTimer) clearTimeout(dropMessageTimer);
		dropMessageTimer = setTimeout(() => {
			dropMessage = '';
			dropMessageTimer = null;
		}, 4200);
	}

	async function addImageFiles(files: File[]) {
		const result = await readImageAttachments(files, images.length);
		if (result.accepted.length > 0) {
			images = [...images, ...result.accepted];
		}
		if (result.rejected.length > 0) {
			const first = result.rejected[0];
			setDropMessage(`${first.name}: ${first.reason}`);
		} else if (result.accepted.length > 0) {
			setDropMessage(
				`Attached ${result.accepted.length} ${result.accepted.length === 1 ? 'image' : 'images'}.`
			);
		}
	}

	function removeImage(id: string | undefined) {
		images = images.filter((image) => image.id !== id);
	}

	function onDragEnter(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		dragDepth += 1;
	}

	function onDragOver(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
	}

	function onDragLeave(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		dragDepth = Math.max(0, dragDepth - 1);
	}

	async function onDrop(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		dragDepth = 0;
		const files = Array.from(e.dataTransfer?.files ?? []);
		if (files.length === 0) return;
		const imageFiles = files.filter(isSupportedImageFile);
		const textFiles = files.filter((file) => !isSupportedImageFile(file));

		dropProcessing = true;
		try {
			if (imageFiles.length > 0) {
				await addImageFiles(imageFiles);
			}
			const result = await readDroppedTextFiles(textFiles);
			if (result.accepted.length > 0) {
				const formatted = formatDroppedFiles(result.accepted);
				const prefix = value.trim() ? '\n\n' : '';
				input?.insertText(`${prefix}${formatted}\n`);
			}

			if (textFiles.length === 0) {
				return;
			}
			if (result.accepted.length === 0) {
				const first = result.rejected[0];
				setDropMessage(
					first ? `No files added. ${first.name}: ${first.reason}` : 'No files added.'
				);
			} else if (result.rejected.length > 0) {
				setDropMessage(
					`Added ${result.accepted.length} ${result.accepted.length === 1 ? 'file' : 'files'}. ${result.rejected.length} skipped.`
				);
			} else {
				setDropMessage(
					`Added ${result.accepted.length} ${result.accepted.length === 1 ? 'file' : 'files'}.`
				);
			}
		} catch (err) {
			setDropMessage(err instanceof Error ? err.message : 'Failed to read dropped files.');
		} finally {
			dropProcessing = false;
		}
	}

	async function focusInput(options?: { position?: 'start' | 'end' }) {
		await tick();
		// Defer past keydown handlers (session shortcuts) so focus sticks and
		// the caret can be placed after layout settles.
		setTimeout(() => {
			const position = options?.position ?? (value.trim() ? 'end' : 'start');
			void input?.focusAsync({ position });
		}, 0);
	}

	type IdleHandle =
		| { type: 'idle'; id: number }
		| { type: 'timeout'; id: ReturnType<typeof setTimeout> };

	function scheduleIdle(cb: () => void): IdleHandle {
		const ric = (
			window as unknown as {
				requestIdleCallback?: (cb: () => void, opts?: { timeout: number }) => number;
			}
		).requestIdleCallback;
		if (typeof ric === 'function') {
			return { type: 'idle', id: ric(cb, { timeout: 1500 }) };
		}
		return { type: 'timeout', id: setTimeout(cb, 400) };
	}

	function cancelIdle(handle: IdleHandle) {
		if (handle.type === 'idle') {
			const cic = (window as unknown as { cancelIdleCallback?: (id: number) => void })
				.cancelIdleCallback;
			cic?.(handle.id);
		} else {
			clearTimeout(handle.id);
		}
	}
</script>

<div
	class="composer"
	role="group"
	aria-label="Message composer"
	class:hero={variant === 'hero'}
	class:dragging={dragActive}
	ondragenter={onDragEnter}
	ondragover={onDragOver}
	ondragleave={onDragLeave}
	ondrop={onDrop}
>
	{#if dragActive}
		<div class="drop-overlay" aria-hidden="true">
			<FileText size={18} stroke-width={1.8} />
			<span>{dropProcessing ? 'Reading files…' : 'Drop text files to add context'}</span>
		</div>
	{/if}

	{#if dropMessage}
		<div class="drop-message" role="status" transition:fade={{ duration: 120 }}>
			{dropMessage}
		</div>
	{/if}

	{#if workspaceMenuOpen}
		<div
			class="skill-command-menu"
			role="listbox"
			aria-label="Workspace paths"
			bind:this={skillMenu}
			transition:fly={{ y: 6, duration: 120 }}
		>
			<div class="workspace-search-hint" aria-hidden="true">
				<Search size={13} stroke-width={2} />
				{#if workspaceSearchQuery}
					<span class="workspace-search-value">{workspaceSearchQuery}</span>
				{:else}
					<span class="workspace-search-placeholder">Type to filter workspaces…</span>
				{/if}
			</div>
			{#if workspacePathsLoading && !workspacePathsLoaded}
				<p class="skill-command-empty">Loading workspaces...</p>
			{:else if filteredWorkspaceOptions.length === 0}
				<p class="skill-command-empty">No matching workspaces.</p>
			{:else}
				{#each filteredWorkspaceOptions as option, index (`${option.kind}:${option.label}:${index}`)}
					<button
						type="button"
						class="skill-command-option"
						class:highlighted={index === workspaceHighlight}
						data-workspace-index={index}
						role="option"
						aria-selected={index === workspaceHighlight}
						onpointerenter={() => {
							workspaceHighlight = index;
						}}
						onclick={() => {
							void selectWorkspaceOption(option);
						}}
					>
						<span class="skill-command-name">{option.label}</span>
						<span class="skill-command-description">{option.description}</span>
					</button>
				{/each}
			{/if}
		</div>
	{:else if modelCommandMenuOpen}
		<div
			class="skill-command-menu model-command-menu"
			role="listbox"
			aria-label="Select model"
			transition:fly={{ y: 6, duration: 120 }}
		>
			<div class="workspace-search-hint" aria-hidden="true">
				<Search size={13} stroke-width={2} />
				{#if modelCommandQuery}
					<span class="workspace-search-value">{modelCommandQuery}</span>
				{:else}
					<span class="workspace-search-placeholder">Type to filter models…</span>
				{/if}
			</div>
			{#if filteredModelCommandOptions.length === 0}
				<p class="skill-command-empty">No matching models.</p>
			{:else}
				{#each groupedModelCommandOptions as group (group.providerId)}
					<div class="model-command-group">
						<p class="slash-group-heading">{group.providerName}</p>
						{#each group.options as option (option.id)}
							{@const flatIndex = filteredModelCommandOptions.indexOf(option)}
							<button
								type="button"
								class="skill-command-option model-command-option"
								class:highlighted={flatIndex === modelCommandHighlight}
								class:is-selected={option.id === modelStore.selected?.id}
								role="option"
								aria-selected={flatIndex === modelCommandHighlight}
								onpointerenter={() => {
									modelCommandHighlight = flatIndex;
								}}
								onclick={() => {
									void selectModelCommandOption(option);
								}}
							>
								<span class="skill-command-name">{option.label}</span>
								<span class="skill-command-description">{option.modelId}</span>
								{#if option.id === modelStore.selected?.id}
									<span class="model-command-check"
										><Check size={14} stroke-width={2} /></span
									>
								{/if}
							</button>
						{/each}
					</div>
				{/each}
			{/if}
		</div>
	{:else if skillMenuOpen}
		<div
			class="skill-command-menu"
			role="listbox"
			aria-label="Skill commands"
			bind:this={skillMenu}
			transition:fly={{ y: 6, duration: 120 }}
		>
			{#if skillsLoading && !skillsLoaded}
				<p class="skill-command-empty">Loading skills...</p>
			{:else if filteredSlashOptions.length === 0}
				<p class="skill-command-empty">No matching skills.</p>
			{:else}
				{#each filteredSlashOptions as option, index (option.kind + ':' + option.name)}
					{#if index === 0 || filteredSlashOptions[index - 1].kind !== option.kind}
						<p class="slash-group-heading">
							{option.kind === 'builtin' ? 'System commands' : 'Skills'}
						</p>
					{/if}
					<button
						type="button"
						class="skill-command-option"
						class:highlighted={index === skillHighlight}
						data-skill-index={index}
						role="option"
						aria-selected={index === skillHighlight}
						onpointerenter={() => (skillHighlight = index)}
						onpointerdown={(e) => {
							e.preventDefault();
							selectSlashOption(option);
						}}
					>
						<span class="skill-command-name">/{option.name}</span>
						<span class="skill-command-description">{option.description}</span>
					</button>
				{/each}
			{/if}
		</div>
	{/if}

	{#if mentionMenuOpen}
		<div
			class="skill-command-menu mention-menu"
			role="listbox"
			aria-label="Workspace files"
			bind:this={mentionMenu}
			transition:fly={{ y: 6, duration: 120 }}
		>
			{#if !fileIndexReady && filteredMentionFiles.length === 0}
				<p class="skill-command-loading">
					<Loader size={13} stroke-width={2} class="mention-spinner" />
					<span>Indexing workspace…</span>
				</p>
			{:else if useServerSearch && mentionServerLoading && filteredMentionFiles.length === 0}
				<p class="skill-command-loading">
					<Loader size={13} stroke-width={2} class="mention-spinner" />
					<span>Searching…</span>
				</p>
			{:else if fileIndex?.error && filteredMentionFiles.length === 0}
				<p class="skill-command-empty">Could not index workspace.</p>
			{:else if filteredMentionFiles.length === 0}
				<p class="skill-command-empty">No matching files.</p>
			{:else}
				{#each filteredMentionFiles as path, index (path)}
					<button
						type="button"
						class="skill-command-option mention-option"
						class:highlighted={index === mentionHighlight}
						data-mention-index={index}
						role="option"
						aria-selected={index === mentionHighlight}
						onpointerenter={() => (mentionHighlight = index)}
						onpointerdown={(e) => {
							e.preventDefault();
							selectMentionFile(path);
						}}
					>
						<FileText size={14} stroke-width={1.8} />
						<span class="mention-path">{path}</span>
					</button>
				{/each}
			{/if}
			{#if mentionTruncated && !mentionQuery.trim()}
				<p class="mention-hint">
					Showing first {fileIndex?.files.length ?? 0}. Type to search all files.
				</p>
			{/if}
		</div>
	{/if}

	{#if queuedCount > 0}
		<div
			class="queue-picker"
			bind:this={queuePicker}
			in:fly={{ y: 4, duration: 140 }}
			out:fly={{ y: 4, duration: 120 }}
		>
			<button
				type="button"
				class="queue-banner"
				class:open={queuePreviewOpen}
				aria-live="polite"
				aria-expanded={queuePreviewOpen}
				aria-controls="queue-preview-panel"
				onclick={toggleQueuePreview}
			>
				<span>{queuedCount} {queuedCount === 1 ? 'message' : 'messages'} queued</span>
				<ChevronDown size={12} class={queuePreviewOpen ? 'expanded' : ''} />
			</button>

			{#if queuePreviewOpen}
				<div
					id="queue-preview-panel"
					class="queue-preview"
					role="region"
					aria-label="Queued messages"
					transition:fly={{ y: -4, duration: 120 }}
				>
					<ul class="queue-preview-list">
						{#each queuedMessages as message, index (message.id)}
							<li class="queue-preview-item">
								<span class="queue-preview-index">{index + 1}</span>
								<p class="queue-preview-text">{message.text}</p>
								<button
									type="button"
									class="queue-remove"
									aria-label={`Remove queued message ${index + 1}`}
									onpointerdown={(e) => {
										e.preventDefault();
										e.stopPropagation();
										removeQueued(message.id);
									}}
								>
									<X size={12} stroke-width={2} />
								</button>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		</div>
	{/if}

	<RichComposerInput
		bind:this={input}
		bind:value
		{skillNames}
		mentionsEnabled={hasWorkspace}
		caretTrail={settingsStore.settings.appearance.caretTrail}
		caretColor={settingsStore.settings.appearance.heroComposer.glowColor}
		onkeydown={onKeydown}
		placeholder={waitingForReply
			? 'Waiting for reply…'
			: variant === 'hero'
				? 'Type something. Anything.'
				: 'Type something…'}
		onfiles={(files) => void addImageFiles(files)}
		onmentionquery={onMentionQuery}
	/>

	{#if images.length > 0}
		<div class="image-attachments" aria-label="Attached images">
			{#each images as image (image.id)}
				<div class="image-attachment">
					<img src={imageDataURL(image)} alt={image.name ?? 'Attached image'} />
					<button
						type="button"
						class="image-remove"
						aria-label={`Remove ${image.name ?? 'image'}`}
						onclick={() => removeImage(image.id)}
					>
						<X size={12} stroke-width={2} />
					</button>
				</div>
			{/each}
		</div>
	{/if}

	<div class="composer-footer">
		<div class="composer-tools">
			<div class="model-picker" onfocusout={closeModelMenu}>
				<button
					class="model-button"
					aria-label="Select model"
					aria-expanded={modelOpen}
					title={modelStore.options.length > 0
						? 'Select model for new chats'
						: 'Enable a model in Settings first'}
					disabled={modelStore.options.length === 0}
					onclick={toggleModelMenu}
				>
					<Sparkles size={14} stroke-width={1.8} />
					<span>{modelStore.selected?.label ?? 'No enabled models'}</span>
					<svg
						width="10"
						height="10"
						viewBox="0 0 10 10"
						fill="currentColor"
						aria-hidden="true"
					>
						<path d="M2 4l3 3 3-3H2z" />
					</svg>
				</button>

				{#if modelOpen}
					<div class="model-menu" transition:fly={{ y: 6, duration: 120 }}>
						<input
							class="model-search"
							bind:this={modelSearchInput}
							bind:value={modelSearch}
							placeholder="Search models..."
							spellcheck="false"
						/>
						{#each groupedModelOptions as group (group.providerId)}
							<div class="model-group" transition:fade={{ duration: 90 }}>
								<div class="model-group-heading">
									<strong>{group.providerName}</strong>
									<small>{group.providerMethod}</small>
								</div>
								{#each group.options as option (option.id)}
									<button
										class="model-option"
										onclick={() => selectModel(option)}
									>
										<span class="model-check">
											{#if option.id === modelStore.selected?.id}<Check
													size={14}
													stroke-width={2}
												/>{/if}
										</span>
										<span class="model-option-copy">
											<strong>{option.label}</strong>
											<small>{option.modelId}</small>
										</span>
									</button>
								{/each}
							</div>
						{:else}
							<p class="model-empty">No enabled models match your search.</p>
						{/each}
					</div>
				{/if}
			</div>
			{#if hasWorkspace}
				<button
					type="button"
					class="workspace-indicator"
					title={shellStore.workspacePath}
					aria-label="Change workspace"
					aria-expanded={workspaceMenuOpen}
					onclick={openChangeWorkspace}
				>
					<Folder size={14} stroke-width={1.8} />
					<span>{currentWorkspaceLabel}</span>
				</button>
			{/if}
		</div>

		<div class="composer-actions">
			{#if streaming}
				<button class="stop-button" onclick={() => onStop?.()} aria-label="Stop response">
					<Square size={14} fill="currentColor" stroke-width={0} />
				</button>
			{:else}
				<button
					class="send-button"
					onclick={submit}
					disabled={!canSubmit || disabled || !modelStore.selected}
					aria-label="Send"
				>
					<Send size={16} stroke-width={1.8} />
				</button>
			{/if}
		</div>
	</div>
</div>

<style>
	.composer {
		position: relative;
		background: rgba(255, 255, 255, 0.74);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		padding: 14px 14px 10px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		backdrop-filter: blur(18px) saturate(170%);
		-webkit-backdrop-filter: blur(18px) saturate(170%);
		transition:
			width var(--duration-flight) var(--ease-smooth),
			padding var(--duration-flight) var(--ease-smooth),
			border-radius var(--duration-flight) var(--ease-smooth),
			box-shadow var(--duration-flight) var(--ease-smooth),
			transform var(--duration-flight) var(--ease-smooth),
			background var(--duration-flight) var(--ease-smooth);
	}

	.composer.dragging {
		border-color: rgba(37, 99, 235, 0.26);
		background: rgba(248, 251, 255, 0.92);
		box-shadow:
			var(--shadow-card),
			0 0 0 4px rgba(37, 99, 235, 0.08);
	}

	.drop-overlay {
		position: absolute;
		inset: 8px;
		z-index: 20;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		border: 1px dashed rgba(37, 99, 235, 0.34);
		border-radius: calc(var(--radius-card) - 6px);
		background: rgba(255, 255, 255, 0.78);
		color: #1d4ed8;
		font-size: 13px;
		font-weight: 600;
		pointer-events: none;
		backdrop-filter: blur(10px);
		-webkit-backdrop-filter: blur(10px);
	}

	.drop-message {
		position: absolute;
		right: 12px;
		bottom: calc(100% + 8px);
		z-index: 25;
		max-width: min(360px, calc(100vw - 32px));
		padding: 7px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.96);
		box-shadow: var(--shadow-card);
		color: var(--text-muted);
		font-size: 12px;
		line-height: 1.35;
	}

	.skill-command-menu {
		position: absolute;
		left: 14px;
		right: 14px;
		bottom: calc(100% + 8px);
		z-index: 28;
		max-height: 260px;
		overflow: auto;
		padding: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(246, 249, 252, 0.98);
		box-shadow: var(--shadow-card);
		scrollbar-gutter: stable;
	}

	.skill-command-option {
		display: flex;
		width: 100%;
		flex-direction: column;
		gap: 3px;
		padding: 9px 10px;
		border: none;
		border-radius: 10px;
		background: transparent;
		text-align: left;
		cursor: pointer;
	}

	.skill-command-option:hover,
	.skill-command-option.highlighted {
		background: rgba(15, 23, 42, 0.06);
	}

	.skill-command-name {
		font-size: 12px;
		font-weight: 700;
		color: var(--text-main);
	}

	.skill-command-description {
		font-size: 11px;
		line-height: 1.35;
		color: var(--text-soft);
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		overflow: hidden;
	}

	.skill-command-empty {
		margin: 0;
		padding: 10px 12px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.skill-command-loading {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 0;
		padding: 10px 12px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.skill-command-loading :global(.mention-spinner) {
		flex-shrink: 0;
		color: var(--text-soft);
		animation: mention-spin 0.7s linear infinite;
	}

	@keyframes mention-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.skill-command-loading :global(.mention-spinner) {
			animation: none;
		}
	}

	.slash-group-heading {
		margin: 0;
		padding: 8px 10px 4px;
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--text-soft);
	}

	.slash-group-heading:first-child {
		padding-top: 4px;
	}

	.workspace-search-hint {
		display: flex;
		align-items: center;
		gap: 7px;
		margin: 2px 2px 6px;
		padding: 7px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 9px;
		background: rgba(255, 255, 255, 0.7);
		color: var(--text-soft);
		font-size: 12px;
		line-height: 1.2;
	}

	.workspace-search-hint :global(svg) {
		flex-shrink: 0;
		color: var(--text-soft);
	}

	.workspace-search-value {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-main);
		font-weight: 500;
	}

	.workspace-search-placeholder {
		color: var(--text-soft);
	}

	.mention-option {
		flex-direction: row;
		align-items: center;
		gap: 8px;
		padding: 7px 10px;
	}

	.mention-option :global(svg) {
		flex-shrink: 0;
		color: var(--text-soft);
	}

	.mention-path {
		font-size: 12px;
		font-weight: 500;
		color: var(--text-main);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.mention-hint {
		margin: 2px 0 0;
		padding: 6px 10px 2px;
		font-size: 10px;
		color: var(--text-soft);
		border-top: 1px solid var(--border-soft);
	}

	.image-attachments {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		margin-top: -2px;
	}

	.image-attachment {
		position: relative;
		width: 58px;
		height: 58px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(15, 23, 42, 0.04);
		overflow: hidden;
	}

	.image-attachment img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}

	.image-remove {
		position: absolute;
		top: 4px;
		right: 4px;
		display: grid;
		place-items: center;
		width: 18px;
		height: 18px;
		padding: 0;
		border: none;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.72);
		color: white;
		cursor: pointer;
	}

	.image-remove:hover {
		background: rgba(180, 35, 24, 0.9);
	}

	.composer.hero {
		padding: 24px 24px 16px;
		border-radius: 24px;
		box-shadow: 0 18px 60px rgba(15, 23, 42, 0.12);
	}

	.composer-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.composer-tools,
	.composer-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.queue-picker {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.queue-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		margin: -2px 0 -4px;
		padding: 6px 10px;
		border: none;
		border-radius: 10px;
		background: rgba(15, 23, 42, 0.04);
		font-size: 11px;
		font-weight: 500;
		line-height: 1.2;
		color: var(--text-soft);
		cursor: pointer;
		text-align: left;
	}

	.queue-banner:hover,
	.queue-banner.open {
		background: rgba(15, 23, 42, 0.07);
		color: var(--text-muted);
	}

	.queue-banner :global(svg) {
		flex-shrink: 0;
		transition: transform var(--duration-fast) var(--ease-smooth);
	}

	.queue-banner :global(.expanded) {
		transform: rotate(180deg);
	}

	.queue-preview {
		padding: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.92);
		box-shadow: var(--shadow-card);
	}

	.queue-preview-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-height: 160px;
		overflow-y: auto;
		scrollbar-gutter: stable;
	}

	.queue-preview-item {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 7px 8px;
		border-radius: 8px;
		background: rgba(15, 23, 42, 0.03);
	}

	.queue-remove {
		flex: 0 0 auto;
		display: grid;
		place-items: center;
		margin-left: auto;
		padding: 4px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-soft);
		cursor: pointer;
	}

	.queue-remove:hover {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.queue-preview-index {
		flex: 0 0 auto;
		font-size: 10px;
		font-weight: 600;
		line-height: 1.45;
		color: var(--text-soft);
	}

	.queue-preview-text {
		margin: 0;
		min-width: 0;
		flex: 1;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
		white-space: pre-wrap;
		word-break: break-word;
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		overflow: hidden;
	}

	.composer-footer button {
		border: none;
		background: transparent;
		color: var(--text-muted);
		border-radius: 7px;
		font-size: 13px;
		cursor: pointer;
	}

	.composer-footer button:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.composer-footer button:active:not(:disabled) {
		background: rgba(0, 0, 0, 0.07);
	}

	.composer-footer button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.send-button {
		display: grid;
		place-items: center;
		padding: 6px;
		color: var(--accent) !important;
	}

	.stop-button {
		display: grid;
		place-items: center;
		padding: 6px;
		color: var(--text-muted) !important;
	}

	.stop-button:hover:not(:disabled) {
		color: #b42318 !important;
		background: rgba(180, 35, 24, 0.08);
	}

	.workspace-indicator,
	.model-button {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		max-width: 100%;
		padding: 5px 8px;
		font-size: 13px;
		font-weight: 500;
		line-height: 1;
		color: var(--text-muted);
		white-space: nowrap;
	}

	.workspace-indicator span,
	.model-button span {
		min-width: 0;
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.workspace-indicator span {
		text-transform: uppercase;
	}

	.workspace-indicator :global(svg),
	.model-button :global(svg:first-child) {
		flex-shrink: 0;
	}

	.model-button svg:last-child {
		flex-shrink: 0;
	}

	.model-picker {
		position: relative;
		min-width: 0;
	}

	.model-button,
	.workspace-indicator {
		border: none;
		background: transparent;
		border-radius: 7px;
		cursor: pointer;
	}

	.model-menu {
		position: absolute;
		left: 0;
		bottom: calc(100% + 8px);
		width: 330px;
		max-height: 420px;
		overflow: auto;
		scrollbar-gutter: stable;
		padding: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(246, 249, 252, 0.98);
		box-shadow: var(--shadow-card);
		z-index: 30;
	}

	.model-search {
		width: 100%;
		border: none;
		border-bottom: 1px solid var(--border-soft);
		background: transparent;
		padding: 10px 12px 12px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		outline: none;
	}

	.model-search::placeholder {
		color: var(--text-soft);
	}

	.model-group {
		padding: 6px 0;
	}

	.model-group-heading {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		color: var(--text-muted);
	}

	.model-group-heading strong {
		font-size: 12px;
		font-weight: 700;
	}

	.model-group-heading small {
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		padding: 2px 7px;
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.02em;
	}

	.model-option {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		padding: 9px 10px;
		text-align: left;
		color: var(--text-main);
	}

	.model-check {
		width: 18px;
		display: grid;
		place-items: center;
		color: var(--text-main);
		flex-shrink: 0;
	}

	.model-option-copy {
		display: flex;
		min-width: 0;
		width: 100%;
		flex-direction: column;
		gap: 2px;
	}

	.model-option-copy strong,
	.model-option-copy small {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.model-option-copy strong {
		font-size: 13px;
		font-weight: 600;
	}

	.model-option-copy small {
		font-size: 11px;
		font-weight: 400;
		color: var(--text-soft);
	}

	.model-empty {
		margin: 0;
		padding: 12px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.model-command-menu {
		max-height: 320px;
		overflow-y: auto;
	}

	.model-command-option {
		position: relative;
	}

	.model-command-option.is-selected {
		background: rgba(0, 102, 204, 0.04);
	}

	.model-command-check {
		position: absolute;
		right: 10px;
		top: 50%;
		transform: translateY(-50%);
		color: rgba(0, 102, 204, 0.7);
	}
</style>
