import { tick } from 'svelte';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import { buildJobExecutionPrompt, claimJob, listJobs } from '$lib/client/cometmind';
import type { ComposerInputRef } from '$lib/features/composer/composer-input-ref';
import { createMenuHighlight } from '$lib/features/composer/menu-highlight.svelte';
import { jobUserDisplayText } from '$lib/features/jobs/format-job-label';
import { filterJobOptions, parseJobCommand } from '$lib/features/skills/slash-commands';
import type { JobResource } from '$lib/generated/cometmind-api';
import { shellStore } from '$lib/stores/shell.svelte';

export function createComposerJobCommands(deps: {
	getValue: () => string;
	setValue: (value: string) => void;
	getInput: () => ComposerInputRef | null;
	getSessionId: () => string;
	sendTurn: (payload: ChatTurnPayload | string) => void;
	setDropMessage: (message: string) => void;
	getSkillMenuRef: () => HTMLDivElement | null;
}) {
	let readyJobs = $state<JobResource[]>([]);
	let jobsLoading = $state(false);
	let jobsLoaded = $state(false);

	const jobCommand = $derived(parseJobCommand(deps.getValue()));
	const jobCommandMenuOpen = $derived(Boolean(jobCommand));
	const jobCommandQuery = $derived(jobCommand?.query ?? '');
	const filteredJobOptions = $derived.by(() => filterJobOptions(jobCommandQuery, readyJobs));
	const highlight = createMenuHighlight({
		getQuery: () => jobCommandQuery,
		getOpen: () => jobCommandMenuOpen,
		getCount: () => filteredJobOptions.length
	});

	$effect(() => {
		if (!jobCommandMenuOpen) return;
		void ensureReadyJobsLoaded();
	});

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

	async function scrollHighlightedIntoView() {
		await tick();
		const option = deps
			.getSkillMenuRef()
			?.querySelector(`[data-job-index="${highlight.index}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
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
		const option = filteredJobOptions[highlight.index];
		if (option) {
			void selectJobCommandOption(option);
			return;
		}
		deps.getInput()?.clear();
		deps.setValue('');
	}

	function handleKeydown(e: KeyboardEvent): boolean {
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
				highlight.index = (highlight.index + 1) % filteredJobOptions.length;
				void scrollHighlightedIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredJobOptions.length > 0) {
				highlight.index =
					(highlight.index - 1 + filteredJobOptions.length) % filteredJobOptions.length;
				void scrollHighlightedIntoView();
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

	function prepareOpen() {
		highlight.index = 0;
		void ensureReadyJobsLoaded();
	}

	return {
		get active() {
			return jobCommand;
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
			return highlight.index;
		},
		set jobCommandHighlight(index: number) {
			highlight.index = index;
		},
		selectJobCommandOption,
		handleJobCommandSubmit,
		handleKeydown,
		prepareOpen
	};
}
