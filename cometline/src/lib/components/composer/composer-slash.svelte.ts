import { tick } from 'svelte';
import { goto } from '$app/navigation';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import {
	listSkills,
	listWorkspaces,
	forkSession,
	clearSession,
	deleteWorkspace,
	listJobs,
	claimJob,
	buildJobExecutionPrompt
} from '$lib/client/cometmind';
import { jobUserDisplayText } from '$lib/jobs/format-job-label';
import { listJobsUserDisplayText } from '$lib/jobs/format-ready-jobs-list';
import { sessionStore } from '$lib/stores/session.svelte';
import { chatStore } from '$lib/stores/chat.svelte';
import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
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
import type { ImageAttachment, SkillResource } from '$lib/types';
import type { JobResource } from '$lib/generated/cometmind-api';
import type { ComposerInputRef } from '$lib/components/composer/composer-input-ref';

export type ComposerSubmitResolution =
	| { kind: 'handled' }
	| { kind: 'message'; text: string };

export function createComposerSlashController(deps: {
	getValue: () => string;
	setValue: (value: string) => void;
	getInput: () => ComposerInputRef | null;
	getSessionId: () => string;
	getStreaming: () => boolean;
	getImages: () => ImageAttachment[];
	setImages: (images: ImageAttachment[]) => void;
	sendTurn: (payload: ChatTurnPayload | string) => void;
	onLocalUserMessage?: (text: string) => void;
	onModelChange?: (option: ModelOption) => void | Promise<void>;
	onWorkspaceChanged?: () => void | Promise<void>;
	onTranscriptCleared?: () => void;
	setDropMessage: (message: string) => void;
	focusInput: (options?: { position?: 'start' | 'end' }) => Promise<void>;
	getSkillMenuRef: () => HTMLDivElement | null;
}) {
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
	let modelCommandHighlight = $state(0);
	let readyJobs = $state<JobResource[]>([]);
	let jobsLoading = $state(false);
	let jobsLoaded = $state(false);
	let jobCommandHighlight = $state(0);

	const skillCommandMatch = $derived(/^\s*\/([\w-]*)$/.exec(deps.getValue()));
	const skillCommandQuery = $derived(skillCommandMatch?.[1]?.toLowerCase() ?? '');
	const skillMenuOpen = $derived(
		Boolean(skillCommandMatch && skillCommandMatch[0] !== dismissedSkillCommand)
	);
	const filteredSlashOptions = $derived.by(() => {
		if (!skillCommandMatch) return [];
		return filterSlashMenuOptions(skillCommandQuery, skills);
	});
	const changeCommand = $derived(parseChangeCommand(deps.getValue()));
	const workspaceMenuOpen = $derived(Boolean(changeCommand));
	const workspaceSearchQuery = $derived(changeCommand?.query ?? '');
	const filteredWorkspaceOptions = $derived.by(() => {
		if (!changeCommand) return [];
		return filterWorkspaceOptions(workspaceSearchQuery, workspacePaths, workspaceSessionCounts);
	});
	const modelCommand = $derived(parseModelCommand(deps.getValue()));
	const modelCommandMenuOpen = $derived(Boolean(modelCommand));
	const modelCommandQuery = $derived(modelCommand?.query ?? '');
	const filteredModelCommandOptions = $derived.by(() => {
		const query = modelCommandQuery.trim().toLowerCase();
		if (!query) return modelStore.options;
		return modelStore.options.filter(
			(option) =>
				option.label.toLowerCase().includes(query) ||
				option.modelId.toLowerCase().includes(query) ||
				option.providerName.toLowerCase().includes(query)
		);
	});
	const groupedModelCommandOptions = $derived.by(() => {
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
	const jobCommand = $derived(parseJobCommand(deps.getValue()));
	const jobCommandMenuOpen = $derived(Boolean(jobCommand));
	const jobCommandQuery = $derived(jobCommand?.query ?? '');
	const filteredJobOptions = $derived.by(() => {
		return filterJobOptions(jobCommandQuery, readyJobs);
	});
	const skillNames = $derived([
		...BUILTIN_SLASH_COMMANDS.map((cmd) => cmd.name),
		...skills.map((skill) => skill.name)
	]);

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
		if (jobCommandMenuOpen) {
			void ensureReadyJobsLoaded();
			jobCommandHighlight = 0;
		}
	});

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

	async function scrollHighlightedWorkspaceIntoView() {
		await tick();
		const option = deps
			.getSkillMenuRef()
			?.querySelector(`[data-workspace-index="${workspaceHighlight}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
	}

	async function scrollHighlightedSkillIntoView() {
		await tick();
		const option = deps
			.getSkillMenuRef()
			?.querySelector(`[data-skill-index="${skillHighlight}"]`);
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
		const sessionId = deps.getSessionId();
		if (!sessionId || deps.getStreaming()) return;
		try {
			await clearSession(sessionId);
			chatStore.resetTranscript(sessionId);
			shellStore.centerComposer();
			deps.onTranscriptCleared?.();
			const current = sessionStore.current;
			if (current?.id === sessionId) {
				sessionStore.updateSession({ ...current, title: '' });
			}
			deps.getInput()?.clear();
			deps.setValue('');
			deps.setImages([]);
			deps.setDropMessage('Cleared conversation history');
			void deps.focusInput();
		} catch (err) {
			deps.setDropMessage(err instanceof Error ? err.message : 'Failed to clear session');
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
			skillsLoaded = false;
			skills = [];
			workspacePathsLoaded = false;
			deps.getInput()?.clear();
			deps.setValue('');
			workspaceHighlight = 0;
			if (forkedId) {
				deps.setDropMessage(`Forked session into ${clean}`);
				await goto(sessionRouteFor(forkedId));
			} else {
				deps.setDropMessage(`Switched workspace to ${clean}`);
				await deps.onWorkspaceChanged?.();
			}
			void deps.focusInput();
		} catch (err) {
			deps.setDropMessage(err instanceof Error ? err.message : 'Failed to fork session');
		}
	}

	function sessionRouteFor(sessionId: string) {
		return window.location.pathname.startsWith('/mini')
			? `/mini/session/${sessionId}`
			: `/session/${sessionId}`;
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

	async function selectModelCommandOption(option: ModelOption) {
		modelStore.select(option);
		await deps.onModelChange?.(option);
		deps.getInput()?.clear();
		deps.setValue('');
		modelCommandHighlight = 0;
		deps.setDropMessage(`Switched to ${option.label}`);
	}

	function handleModelCommandSubmit() {
		const flatOptions = filteredModelCommandOptions;
		const option = flatOptions[modelCommandHighlight];
		if (option) {
			void selectModelCommandOption(option);
			return;
		}
		deps.getInput()?.clear();
		deps.setValue('');
	}

	async function selectJobCommandOption(job: JobResource) {
		const sessionId = deps.getSessionId();
		if (!sessionId) return;
		try {
			const claimed = await claimJob(job.id, sessionId);
			let prompt = buildJobExecutionPrompt(claimed);
			const jobPath = claimed.workspace_path?.trim();
			const sessionPath = shellStore.workspacePath?.trim();
			if (jobPath && sessionPath && jobPath !== sessionPath) {
				prompt += `\n\nNote: this job targets workspace \`${jobPath}\` but this session uses \`${sessionPath}\`. Consider /change to fork into the correct workspace before editing files.`;
			}
			deps.getInput()?.clear();
			deps.setValue('');
			deps.sendTurn({ text: prompt, displayText: jobUserDisplayText(claimed) });
		} catch (err) {
			deps.setDropMessage(err instanceof Error ? err.message : 'Failed to claim job');
		}
	}

	function handleJobCommandSubmit() {
		const option = filteredJobOptions[jobCommandHighlight];
		if (option) {
			void selectJobCommandOption(option);
			return;
		}
		deps.getInput()?.clear();
		deps.setValue('');
	}

	async function handleListJobsSubmit() {
		deps.getInput()?.clear();
		deps.setValue('');
		try {
			const res = await listJobs({ ready_only: true });
			const text = listJobsUserDisplayText(res.jobs ?? []);
			deps.onLocalUserMessage?.(text);
		} catch (err) {
			deps.setDropMessage(err instanceof Error ? err.message : 'Failed to list jobs');
		}
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

	function resolveSubmitAction(trimmed: string): ComposerSubmitResolution {
		if (isChangeWorkspaceCommand(trimmed)) {
			void handleChangeWorkspaceSubmit(trimmed);
			return { kind: 'handled' };
		}
		if (parseClearCommand(trimmed)) {
			void handleClearSubmit();
			return { kind: 'handled' };
		}
		if (parseListJobsCommand(trimmed)) {
			void handleListJobsSubmit();
			return { kind: 'handled' };
		}
		if (modelCommand) {
			handleModelCommandSubmit();
			return { kind: 'handled' };
		}
		if (jobCommand) {
			handleJobCommandSubmit();
			return { kind: 'handled' };
		}
		const expanded = expandBuiltinSlashCommand(trimmed) ?? expandSkillCommand(trimmed);
		return { kind: 'message', text: expanded };
	}

	function handleWorkspaceMenuKeydown(e: KeyboardEvent): boolean {
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
			deps.getInput()?.clear();
			deps.setValue('');
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

	function handleJobCommandMenuKeydown(e: KeyboardEvent): boolean {
		if (!jobCommandMenuOpen) return false;
		if (e.key === 'Escape') {
			e.preventDefault();
			deps.getInput()?.clear();
			deps.setValue('');
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
					(jobCommandHighlight - 1 + filteredJobOptions.length) %
					filteredJobOptions.length;
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
			dismissedSkillCommand = skillCommandMatch?.[0] ?? deps.getValue();
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

	function handleMenuKeydown(e: KeyboardEvent): boolean {
		if (handleWorkspaceMenuKeydown(e)) return true;
		if (handleModelCommandMenuKeydown(e)) return true;
		if (handleJobCommandMenuKeydown(e)) return true;
		if (handleSkillMenuKeydown(e)) return true;
		return false;
	}

	function openChangeWorkspace() {
		const next = '/change ';
		deps.getInput()?.setText(next);
		deps.setValue(next);
		dismissedSkillCommand = '';
		skillHighlight = 0;
		workspaceHighlight = 0;
		void ensureWorkspacePathsLoaded();
		void deps.focusInput();
	}

	function selectSlashOption(option: SlashMenuOption) {
		if (option.kind === 'builtin' && option.name === 'change') {
			openChangeWorkspace();
			return;
		}
		if (option.kind === 'builtin' && option.name === 'list-jobs') {
			deps.getInput()?.clear();
			deps.setValue('');
			void handleListJobsSubmit();
			return;
		}
		const next = `/${option.name} `;
		deps.getInput()?.setText(next);
		deps.setValue(next);
		dismissedSkillCommand = next;
		skillHighlight = 0;
	}

	return {
		get skillNames() {
			return skillNames;
		},
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
			return workspaceHighlight;
		},
		set workspaceHighlight(index: number) {
			workspaceHighlight = index;
		},
		get workspaceDeleting() {
			return workspaceDeleting;
		},
		get modelCommandMenuOpen() {
			return modelCommandMenuOpen;
		},
		get modelCommandQuery() {
			return modelCommandQuery;
		},
		get filteredModelCommandOptions() {
			return filteredModelCommandOptions;
		},
		get groupedModelCommandOptions() {
			return groupedModelCommandOptions;
		},
		get modelCommandHighlight() {
			return modelCommandHighlight;
		},
		set modelCommandHighlight(index: number) {
			modelCommandHighlight = index;
		},
		get jobCommandMenuOpen() {
			return jobCommandMenuOpen;
		},
		get jobCommandQuery() {
			return jobCommandQuery;
		},
		get jobsLoading() {
			return jobsLoading;
		},
		get jobsLoaded() {
			return jobsLoaded;
		},
		get filteredJobOptions() {
			return filteredJobOptions;
		},
		get jobCommandHighlight() {
			return jobCommandHighlight;
		},
		set jobCommandHighlight(index: number) {
			jobCommandHighlight = index;
		},
		get skillMenuOpen() {
			return skillMenuOpen;
		},
		get skillsLoading() {
			return skillsLoading;
		},
		get skillsLoaded() {
			return skillsLoaded;
		},
		get filteredSlashOptions() {
			return filteredSlashOptions;
		},
		get skillHighlight() {
			return skillHighlight;
		},
		set skillHighlight(index: number) {
			skillHighlight = index;
		},
		resolveSubmitAction,
		handleMenuKeydown,
		openChangeWorkspace,
		selectSlashOption,
		selectWorkspaceOption,
		removeWorkspaceFromList,
		selectModelCommandOption,
		selectJobCommandOption
	};
}
