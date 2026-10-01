import { tick } from 'svelte';
import { createMenuHighlight } from '$lib/features/composer/menu-highlight.svelte';
import { groupModelCommandOptions } from '$lib/features/composer/composer-model-groups';
import { createComposerJobCommands } from '$lib/features/composer/composer-slash-jobs.svelte';
import { createComposerWorkspaceCommands } from '$lib/features/composer/composer-slash-workspace.svelte';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import { clearSession, listSkills } from '$lib/client/cometmind';
import { chatStore } from '$lib/stores/chat.svelte';
import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import {
	BUILTIN_SLASH_COMMANDS,
	expandBuiltinSlashCommand,
	filterSlashMenuOptions,
	isChangeWorkspaceCommand,
	isJobCommand,
	isModelCommand,
	parseChangeCommand,
	parseClearCommand,
	parseModelCommand,
	type SlashMenuOption
} from '$lib/features/skills/slash-commands';
import type { ImageAttachment, SkillResource } from '$lib/types';
import type { ComposerInputRef } from '$lib/features/composer/composer-input-ref';

export type ComposerSubmitResolution =
	| { kind: 'handled' }
	| { kind: 'message'; text: string; displayText?: string };

export function createComposerSlashController(deps: {
	getValue: () => string;
	setValue: (value: string) => void;
	getInput: () => ComposerInputRef | null;
	getSessionId: () => string;
	getStreaming: () => boolean;
	getImages: () => ImageAttachment[];
	setImages: (images: ImageAttachment[]) => void;
	sendTurn: (payload: ChatTurnPayload | string) => void;
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
	let dismissedSkillCommand = $state('');

	const workspace = createComposerWorkspaceCommands({
		getValue: deps.getValue,
		setValue: deps.setValue,
		getInput: deps.getInput,
		getSessionId: deps.getSessionId,
		onWorkspaceChanged: deps.onWorkspaceChanged,
		setDropMessage: deps.setDropMessage,
		focusInput: deps.focusInput,
		getSkillMenuRef: deps.getSkillMenuRef,
		invalidateSkills: () => {
			skillsLoaded = false;
			skills = [];
		}
	});
	const jobs = createComposerJobCommands({
		getValue: deps.getValue,
		setValue: deps.setValue,
		getInput: deps.getInput,
		getSessionId: deps.getSessionId,
		sendTurn: deps.sendTurn,
		setDropMessage: deps.setDropMessage,
		getSkillMenuRef: deps.getSkillMenuRef
	});

	const skillCommandMatch = $derived(/^\s*\/([\w-]*)$/.exec(deps.getValue()));
	const skillCommandQuery = $derived(skillCommandMatch?.[1]?.toLowerCase() ?? '');
	const skillMenuOpen = $derived(
		Boolean(skillCommandMatch && skillCommandMatch[0] !== dismissedSkillCommand)
	);
	const filteredSlashOptions = $derived.by(() => {
		if (!skillCommandMatch) return [];
		return filterSlashMenuOptions(skillCommandQuery, skills);
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
	const groupedModelCommandOptions = $derived(
		groupModelCommandOptions(filteredModelCommandOptions)
	);
	const skillNames = $derived([
		...BUILTIN_SLASH_COMMANDS.map((cmd) => cmd.name),
		...skills.map((skill) => skill.name)
	]);
	const skillHighlightMenu = createMenuHighlight({
		getQuery: () => skillCommandQuery,
		getOpen: () => skillMenuOpen,
		getCount: () => filteredSlashOptions.length
	});
	const modelCommandHighlightMenu = createMenuHighlight({
		getQuery: () => modelCommandQuery,
		getOpen: () => modelCommandMenuOpen,
		getCount: () => filteredModelCommandOptions.length
	});
	$effect(() => {
		if (!skillCommandMatch) {
			dismissedSkillCommand = '';
			return;
		}
		void ensureSkillsLoaded();
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

	async function scrollHighlightedSkillIntoView() {
		await tick();
		const option = deps
			.getSkillMenuRef()
			?.querySelector(`[data-skill-index="${skillHighlightMenu.index}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
	}

	async function scrollHighlightedModelIntoView() {
		await tick();
		const option = deps
			.getSkillMenuRef()
			?.querySelector(`[data-model-index="${modelCommandHighlightMenu.index}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
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
			deps.getInput()?.clear();
			deps.setValue('');
			deps.setImages([]);
			deps.setDropMessage('Cleared conversation history');
			void deps.focusInput();
		} catch (err) {
			deps.setDropMessage(err instanceof Error ? err.message : 'Failed to clear session');
		}
	}

	async function selectModelCommandOption(option: ModelOption) {
		modelStore.select(option);
		await deps.onModelChange?.(option);
		deps.getInput()?.clear();
		deps.setValue('');
		modelCommandHighlightMenu.index = 0;
		deps.setDropMessage(`Switched to ${option.label}`);
	}

	function handleModelCommandSubmit() {
		const flatOptions = filteredModelCommandOptions;
		const option = flatOptions[modelCommandHighlightMenu.index];
		if (option) {
			void selectModelCommandOption(option);
			return;
		}
		deps.getInput()?.clear();
		deps.setValue('');
	}

	function parseLeadingSkillCommand(text: string) {
		const match = /^\s*\/([\w-]+)(?:\s+([\s\S]*))?$/.exec(text);
		if (!match) return null;
		const skillName = match[1];
		if (!skills.some((skill) => skill.name === skillName)) return null;
		return { skillName, rest: match[2]?.trimStart() ?? '' };
	}

	function expandSkillCommand(text: string): { text: string; displayText?: string } {
		const command = parseLeadingSkillCommand(text);
		if (!command) return { text };
		const rest = command.rest ? `\n\n${command.rest}` : '';
		const expanded = `Use the \`${command.skillName}\` skill for this request. Load it with the \`load_skill\` tool before proceeding.${rest}`;
		const displayText = command.rest
			? `/${command.skillName} ${command.rest}`
			: `/${command.skillName}`;
		return { text: expanded, displayText };
	}

	function resolveSubmitAction(trimmed: string): ComposerSubmitResolution {
		if (isChangeWorkspaceCommand(trimmed)) {
			if (!parseChangeCommand(trimmed)) {
				openChangeWorkspace();
				return { kind: 'handled' };
			}
			void workspace.handleChangeWorkspaceSubmit(trimmed);
			return { kind: 'handled' };
		}
		if (parseClearCommand(trimmed)) {
			void handleClearSubmit();
			return { kind: 'handled' };
		}
		if (isModelCommand(trimmed)) {
			if (!modelCommand) {
				openModelCommand();
				return { kind: 'handled' };
			}
			handleModelCommandSubmit();
			return { kind: 'handled' };
		}
		if (isJobCommand(trimmed)) {
			if (!jobs.active) {
				openJobCommand();
				return { kind: 'handled' };
			}
			jobs.handleJobCommandSubmit();
			return { kind: 'handled' };
		}
		const builtin = expandBuiltinSlashCommand(trimmed);
		if (builtin) {
			return { kind: 'message', text: builtin.text, displayText: builtin.displayText };
		}
		const skill = expandSkillCommand(trimmed);
		return { kind: 'message', text: skill.text, displayText: skill.displayText };
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
				modelCommandHighlightMenu.index =
					(modelCommandHighlightMenu.index + 1) % flatOptions.length;
				void scrollHighlightedModelIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (flatOptions.length > 0) {
				modelCommandHighlightMenu.index =
					(modelCommandHighlightMenu.index - 1 + flatOptions.length) % flatOptions.length;
				void scrollHighlightedModelIntoView();
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const option = flatOptions[modelCommandHighlightMenu.index];
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
				skillHighlightMenu.index =
					(skillHighlightMenu.index + 1) % filteredSlashOptions.length;
				void scrollHighlightedSkillIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredSlashOptions.length > 0) {
				skillHighlightMenu.index =
					(skillHighlightMenu.index - 1 + filteredSlashOptions.length) %
					filteredSlashOptions.length;
				void scrollHighlightedSkillIntoView();
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const option = filteredSlashOptions[skillHighlightMenu.index];
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
		if (workspace.handleKeydown(e)) return true;
		if (handleModelCommandMenuKeydown(e)) return true;
		if (jobs.handleKeydown(e)) return true;
		if (handleSkillMenuKeydown(e)) return true;
		return false;
	}

	function openChangeWorkspace() {
		const next = '/change ';
		deps.getInput()?.setText(next);
		deps.setValue(next);
		dismissedSkillCommand = '';
		skillHighlightMenu.index = 0;
		workspace.prepareOpen();
		void deps.focusInput();
	}

	function openModelCommand() {
		const next = '/model ';
		deps.getInput()?.setText(next);
		deps.setValue(next);
		dismissedSkillCommand = next;
		skillHighlightMenu.index = 0;
		modelCommandHighlightMenu.index = 0;
		void deps.focusInput();
	}

	function openJobCommand() {
		const next = '/job ';
		deps.getInput()?.setText(next);
		deps.setValue(next);
		dismissedSkillCommand = next;
		skillHighlightMenu.index = 0;
		jobs.prepareOpen();
		void deps.focusInput();
	}

	function selectSlashOption(option: SlashMenuOption) {
		if (option.kind === 'builtin' && option.name === 'change') {
			openChangeWorkspace();
			return;
		}
		if (option.kind === 'builtin' && option.name === 'model') {
			openModelCommand();
			return;
		}
		if (option.kind === 'builtin' && option.name === 'job') {
			openJobCommand();
			return;
		}
		const next = `/${option.name} `;
		deps.getInput()?.setText(next);
		deps.setValue(next);
		dismissedSkillCommand = next;
		skillHighlightMenu.index = 0;
	}

	return {
		get skillNames() {
			return skillNames;
		},
		get workspaceMenuOpen() {
			return workspace.workspaceMenuOpen;
		},
		get workspaceSearchQuery() {
			return workspace.workspaceSearchQuery;
		},
		get workspacePathsLoading() {
			return workspace.workspacePathsLoading;
		},
		get workspacePathsLoaded() {
			return workspace.workspacePathsLoaded;
		},
		get filteredWorkspaceOptions() {
			return workspace.filteredWorkspaceOptions;
		},
		get workspaceHighlight() {
			return workspace.workspaceHighlight;
		},
		set workspaceHighlight(index: number) {
			workspace.workspaceHighlight = index;
		},
		get workspaceDeleting() {
			return workspace.workspaceDeleting;
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
			return modelCommandHighlightMenu.index;
		},
		set modelCommandHighlight(index: number) {
			modelCommandHighlightMenu.index = index;
		},
		get jobCommandMenuOpen() {
			return jobs.jobCommandMenuOpen;
		},
		get jobCommandQuery() {
			return jobs.jobCommandQuery;
		},
		get jobsLoading() {
			return jobs.jobsLoading;
		},
		get jobsLoaded() {
			return jobs.jobsLoaded;
		},
		get filteredJobOptions() {
			return jobs.filteredJobOptions;
		},
		get jobCommandHighlight() {
			return jobs.jobCommandHighlight;
		},
		set jobCommandHighlight(index: number) {
			jobs.jobCommandHighlight = index;
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
			return skillHighlightMenu.index;
		},
		set skillHighlight(index: number) {
			skillHighlightMenu.index = index;
		},
		resolveSubmitAction,
		handleMenuKeydown,
		openChangeWorkspace,
		selectSlashOption,
		selectWorkspaceOption: workspace.selectWorkspaceOption,
		removeWorkspaceFromList: workspace.removeWorkspaceFromList,
		selectModelCommandOption,
		selectJobCommandOption: jobs.selectJobCommandOption
	};
}
