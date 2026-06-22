<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { fade } from 'svelte/transition';
	import { Check, FileText, Folder, Loader, Search, Send, Square, Trash2, X } from '@lucide/svelte';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';
	import ContextWindowRing from '$lib/components/composer/ContextWindowRing.svelte';
	import RichComposerInput from '$lib/components/RichComposerInput.svelte';
	import ImageAttachments from '$lib/components/composer/ImageAttachments.svelte';
	import MessageQueuePanel from '$lib/components/composer/MessageQueuePanel.svelte';
	import ModelPicker from '$lib/components/composer/ModelPicker.svelte';
	import SlashCommandMenu from '$lib/components/composer/SlashCommandMenu.svelte';
	import { listSkills, listWorkspaces, forkSession, clearSession, deleteWorkspace, listJobs, claimJob, buildJobExecutionPrompt } from '$lib/client/cometmind';
	import {
		filterFileIndex,
		getFileIndex,
		isFileIndexFresh,
		isFileIndexReady,
		refreshFileIndex,
		searchWorkspaceFiles
	} from '$lib/workspace/file-index';
	import { jobMenuSubtitle, jobUserDisplayText } from '$lib/jobs/format-job-label';
	import { listJobsUserDisplayText } from '$lib/jobs/format-ready-jobs-list';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { chatStore } from '$lib/stores/chat.svelte';
	import {
		BUILTIN_SLASH_COMMANDS,
		expandBuiltinSlashCommand,
		filterSlashMenuOptions,
		filterWorkspaceOptions,
		isChangeWorkspaceCommand,
		parseChangeCommand,
		parseClearCommand,
		parseModelCommand,
		parseJobCommand,
		parseListJobsCommand,
		filterJobOptions,
		type SlashMenuOption,
		type WorkspaceMenuOption
	} from '$lib/skills/slash-commands';
	import { formatDroppedFiles, readDroppedTextFiles } from '$lib/files/dropped-files';
	import {
		estimateChatContextTokens,
		estimateTokensFromText,
		resolveContextWindow
	} from '$lib/context-window';
	import { workspaceLabel } from '$lib/sessions/group-by-workspace';
	import { isSupportedImageFile, readImageAttachments } from '$lib/files/images';
	import type { ImageAttachment, SkillResource } from '$lib/types';
	import type { JobResource } from '$lib/generated/cometmind-api';

	let {
		onSend,
		onLocalUserMessage,
		onStop,
		onRemoveQueued,
		onModelChange,
		onWorkspaceChanged,
		onTranscriptCleared,
		sessionId = '',
		disabled = false,
		streaming = false,
		queuedCount = 0,
		queuedMessages = [],
		waitingForReply = false,
		variant = 'dock',
		autofocus = true
	}: {
		onSend: (payload: ChatTurnPayload | string) => void;
		onLocalUserMessage?: (text: string) => void;
		onStop?: () => void;
		onRemoveQueued?: (id: string) => void;
		onModelChange?: (option: ModelOption) => void | Promise<void>;
		onWorkspaceChanged?: () => void | Promise<void>;
		onTranscriptCleared?: () => void;
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
	let skillMenu = $state<HTMLDivElement | null>(null);
	let skills = $state<SkillResource[]>([]);
	let skillsLoaded = $state(false);
	let skillsLoading = $state(false);
	let skillHighlight = $state(0);
	let workspaceHighlight = $state(0);
	let workspacePaths = $state<string[]>([]);
	let workspaceSessionCounts = $state<Map<string, number>>(new Map());
	let workspacePathsLoading = $state(false);
	let workspacePathsLoaded = $state(false);
	let workspaceDeleting = $state(false);
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
	let contextWindowUsage = $derived.by(() => {
		const limit = resolveContextWindow(settingsStore.settings.cometmind.contextWindowLimit);
		const items =
			sessionId && chatStore.sessionID === sessionId ? chatStore.items : [];
		const draftTokens = value.trim() ? estimateTokensFromText(value) : 0;
		const used = estimateChatContextTokens(items) + draftTokens;
		return { used, limit };
	});
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
		return filterWorkspaceOptions(workspaceSearchQuery, workspacePaths, workspaceSessionCounts);
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
	let readyJobs = $state<JobResource[]>([]);
	let jobsLoading = $state(false);
	let jobsLoaded = $state(false);
	let jobCommand = $derived(parseJobCommand(value));
	let jobCommandMenuOpen = $derived(Boolean(jobCommand));
	let jobCommandQuery = $derived(jobCommand?.query ?? '');
	let jobCommandHighlight = $state(0);
	let filteredJobOptions = $derived.by(() => {
		return filterJobOptions(jobCommandQuery, readyJobs);
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

	onDestroy(() => {
		if (dropMessageTimer) clearTimeout(dropMessageTimer);
	});

	function sendTurn(payload: ChatTurnPayload | string) {
		if (typeof payload === 'string') {
			onSend({ text: payload });
			return;
		}
		onSend(payload);
	}

	function submit() {
		const trimmed = value.trim();
		if (isChangeWorkspaceCommand(trimmed)) {
			void handleChangeWorkspaceSubmit(trimmed);
			return;
		}
		if (parseClearCommand(trimmed)) {
			void handleClearSubmit();
			return;
		}
		if (parseListJobsCommand(trimmed)) {
			void handleListJobsSubmit();
			return;
		}
		if (modelCommand) {
			void handleModelCommandSubmit();
			return;
		}
		if (jobCommand) {
			void handleJobCommandSubmit();
			return;
		}
		const expanded = expandBuiltinSlashCommand(trimmed) ?? expandSkillCommand(trimmed);
		if (!canSubmit || disabled || !modelStore.selected) return;
		const filePaths = input?.getFilePaths() ?? [];
		sendTurn({
			text: expanded,
			images: images.length > 0 ? images : undefined,
			filePaths: filePaths.length > 0 ? filePaths : undefined
		});
		input?.clear();
		value = '';
		images = [];
	}

	async function handleListJobsSubmit() {
		input?.clear();
		value = '';
		try {
			const res = await listJobs({ ready_only: true });
			const text = listJobsUserDisplayText(res.jobs ?? []);
			onLocalUserMessage?.(text);
		} catch (err) {
			dropMessage = err instanceof Error ? err.message : 'Failed to list jobs';
		}
	}

	function onKeydown(e: KeyboardEvent) {
		if (handleWorkspaceMenuKeydown(e)) return;
		if (handleModelCommandMenuKeydown(e)) return;
		if (handleJobCommandMenuKeydown(e)) return;
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

	async function handleClearSubmit() {
		if (!sessionId || streaming) return;
		try {
			await clearSession(sessionId);
			chatStore.resetTranscript(sessionId);
			shellStore.centerComposer();
			onTranscriptCleared?.();
			const current = sessionStore.current;
			if (current?.id === sessionId) {
				sessionStore.updateSession({ ...current, title: '' });
			}
			input?.clear();
			value = '';
			images = [];
			setDropMessage('Cleared conversation history');
			void focusInput();
		} catch (err) {
			setDropMessage(err instanceof Error ? err.message : 'Failed to clear session');
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
			shellStore.commitActiveWorkspace(clean);
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
			setDropMessage(`Removed ${path} from workspace list`);
		} catch (err) {
			setDropMessage(err instanceof Error ? err.message : 'Failed to remove workspace');
		} finally {
			workspaceDeleting = false;
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

	async function ensureReadyJobsLoaded() {
		if (jobsLoaded || jobsLoading) return;
		jobsLoading = true;
		try {
			const res = await listJobs({ ready_only: true });
			readyJobs = res.jobs ?? [];
			jobsLoaded = true;
		} catch {
			readyJobs = [];
			jobsLoaded = true;
		} finally {
			jobsLoading = false;
		}
	}

	$effect(() => {
		if (jobCommandMenuOpen) {
			void ensureReadyJobsLoaded();
			jobCommandHighlight = 0;
		}
	});

	async function selectJobCommandOption(job: JobResource) {
		if (!sessionId) return;
		try {
			const claimed = await claimJob(job.id, sessionId);
			let prompt = buildJobExecutionPrompt(claimed);
			const jobPath = claimed.workspace_path?.trim();
			const sessionPath = shellStore.workspacePath?.trim();
			if (jobPath && sessionPath && jobPath !== sessionPath) {
				prompt +=
					`\n\nNote: this job targets workspace \`${jobPath}\` but this session uses \`${sessionPath}\`. Consider /change to fork into the correct workspace before editing files.`;
			}
			input?.clear();
			value = '';
			sendTurn({ text: prompt, displayText: jobUserDisplayText(claimed) });
		} catch (err) {
			dropMessage = err instanceof Error ? err.message : 'Failed to claim job';
		}
	}

	function handleJobCommandSubmit() {
		const option = filteredJobOptions[jobCommandHighlight];
		if (option) {
			void selectJobCommandOption(option);
			return;
		}
		input?.clear();
		value = '';
	}

	function handleJobCommandMenuKeydown(e: KeyboardEvent): boolean {
		if (!jobCommandMenuOpen) return false;
		if (e.key === 'Escape') {
			e.preventDefault();
			input?.clear();
			value = '';
			return true;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (filteredJobOptions.length > 0) {
				jobCommandHighlight = (jobCommandHighlight + 1) % filteredJobOptions.length;
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredJobOptions.length > 0) {
				jobCommandHighlight =
					(jobCommandHighlight - 1 + filteredJobOptions.length) % filteredJobOptions.length;
			}
			return true;
		}
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			void handleJobCommandSubmit();
			return true;
		}
		return false;
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
		if (option.kind === 'builtin' && option.name === 'list-jobs') {
			input?.clear();
			value = '';
			void handleListJobsSubmit();
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
		<SlashCommandMenu ariaLabel="Workspace paths" bind:menuRef={skillMenu}>
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
					{#if option.kind === 'workspace'}
						<div
							class="workspace-option-row"
							class:highlighted={index === workspaceHighlight}
							data-workspace-index={index}
							role="presentation"
							onpointerenter={() => {
								workspaceHighlight = index;
							}}
						>
							<button
								type="button"
								class="skill-command-option"
								class:highlighted={index === workspaceHighlight}
								role="option"
								aria-selected={index === workspaceHighlight}
								onclick={() => {
									void selectWorkspaceOption(option);
								}}
							>
								<span class="skill-command-name">{option.label}</span>
								<span class="skill-command-description">{option.description}</span>
							</button>
							{#if option.deletable}
								<button
									type="button"
									class="workspace-delete-btn"
									aria-label={`Remove ${option.label} from workspace list`}
									disabled={workspaceDeleting}
									onclick={(event) => {
										void removeWorkspaceFromList(option.path, event);
									}}
								>
									<Trash2 size={13} stroke-width={2} />
								</button>
							{/if}
						</div>
					{:else}
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
					{/if}
				{/each}
			{/if}
		</SlashCommandMenu>
	{:else if modelCommandMenuOpen}
		<SlashCommandMenu ariaLabel="Select model" class="model-command-menu">
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
		</SlashCommandMenu>
	{:else if jobCommandMenuOpen}
		<SlashCommandMenu ariaLabel="Select job" class="job-command-menu">
			<div class="workspace-search-hint" aria-hidden="true">
				<Search size={13} stroke-width={2} />
				{#if jobCommandQuery}
					<span class="workspace-search-value">{jobCommandQuery}</span>
				{:else}
					<span class="workspace-search-placeholder">Type to filter jobs…</span>
				{/if}
			</div>
			{#if jobsLoading && !jobsLoaded}
				<p class="skill-command-empty">Loading jobs…</p>
			{:else if filteredJobOptions.length === 0}
				<p class="skill-command-empty">No ready jobs.</p>
			{:else}
				{#each filteredJobOptions as job, index (job.id)}
					<button
						type="button"
						class="skill-command-option"
						class:highlighted={index === jobCommandHighlight}
						role="option"
						aria-selected={index === jobCommandHighlight}
						onpointerenter={() => {
							jobCommandHighlight = index;
						}}
						onclick={() => {
							void selectJobCommandOption(job);
						}}
					>
						<span class="skill-command-name">{job.description}</span>
						{#if jobMenuSubtitle(job)}
							<span class="skill-command-description">{jobMenuSubtitle(job)}</span>
						{/if}
					</button>
				{/each}
			{/if}
		</SlashCommandMenu>
	{:else if skillMenuOpen}
		<SlashCommandMenu ariaLabel="Skill commands" bind:menuRef={skillMenu}>
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
		</SlashCommandMenu>
	{/if}

	{#if mentionMenuOpen}
		<SlashCommandMenu
			ariaLabel="Workspace files"
			class="mention-menu"
			bind:menuRef={mentionMenu}
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
		</SlashCommandMenu>
	{/if}

	<MessageQueuePanel {queuedCount} {queuedMessages} onRemove={removeQueued} />

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

	<ImageAttachments {images} onRemove={removeImage} />

	<div class="composer-footer">
		<div class="composer-tools">
			<ModelPicker {onModelChange} />
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
			{#if contextWindowUsage}
				<ContextWindowRing
					usedTokens={contextWindowUsage.used}
					limitTokens={contextWindowUsage.limit}
				/>
			{/if}
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
		border-radius: 999px;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 58%,
			var(--accent, #0066cc)
		) !important;
		transition:
			color 160ms ease,
			background 160ms ease,
			box-shadow 160ms ease;
	}

	.send-button:hover:not(:disabled) {
		color: var(--hero-composer-glow-color, #72c0ff) !important;
		background: var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24)) !important;
		box-shadow: 0 0 14px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.send-button:active:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 22%,
			transparent
		) !important;
		box-shadow: 0 0 8px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.stop-button {
		display: grid;
		place-items: center;
		padding: 6px;
		border-radius: 999px;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 58%,
			var(--accent, #0066cc)
		) !important;
		transition:
			color 160ms ease,
			background 160ms ease,
			box-shadow 160ms ease;
	}

	.stop-button:hover:not(:disabled) {
		color: var(--hero-composer-glow-color, #72c0ff) !important;
		background: var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24)) !important;
		box-shadow: 0 0 14px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.stop-button:active:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, #72c0ff) 22%,
			transparent
		) !important;
		box-shadow: 0 0 8px var(--hero-composer-glow-ring, rgba(114, 192, 255, 0.14));
	}

	.workspace-indicator {
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
		border: none;
		background: transparent;
		border-radius: 7px;
		cursor: pointer;
	}

	.workspace-indicator span {
		min-width: 0;
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
		text-transform: uppercase;
	}

	.workspace-indicator :global(svg) {
		flex-shrink: 0;
	}
</style>
