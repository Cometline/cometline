import {
	createScheduledJob,
	deleteScheduledJob,
	listScheduledJobs,
	updateScheduledJob,
	type CreateScheduledJobRequest,
	type ScheduledJobResource,
	type UpdateScheduledJobRequest
} from '$lib/client/cometmind';
import { truncateJobLabel } from '$lib/features/jobs/format-job-label';
import {
	buildCronExpression,
	localDatetimeToMillis,
	millisToLocalDatetime,
	parseSupportedCron,
	summarizeCronParts,
	type ScheduleFrequency,
	type ScheduleMode
} from '$lib/features/jobs/jobs-page-cron';

export function createJobsScheduleController(deps: {
	isScheduledView: () => boolean;
	getWorkspacePath: () => string | undefined;
	setError: (message: string) => void;
	setSaving: (saving: boolean) => void;
}) {
	let scheduledJobs = $state<ScheduledJobResource[]>([]);
	let showForm = $state(false);
	let editingId = $state('');
	let description = $state('');
	let dod = $state('');
	let workspacePath = $state('');
	let runAtLocal = $state('');
	let mode = $state<ScheduleMode>('one-shot');
	let frequency = $state<ScheduleFrequency>('daily');
	let time = $state('09:00');
	let weekday = $state('1');
	let monthDay = $state('1');
	let unsupportedCron = $state('');

	const generatedCron = $derived(buildCronExpression(time, frequency, weekday, monthDay));
	const summary = $derived(
		mode === 'recurring' ? summarizeCronParts(frequency, time, weekday, monthDay) : ''
	);

	async function loadScheduledJobs(options: { silent?: boolean } = {}) {
		try {
			const res = await listScheduledJobs();
			scheduledJobs = res.scheduled_jobs ?? [];
		} catch (e) {
			if (!options.silent) scheduledJobs = [];
			if (!options.silent || deps.isScheduledView()) {
				deps.setError(e instanceof Error ? e.message : 'Failed to load scheduled jobs');
			}
		}
	}

	function resetForm() {
		editingId = '';
		description = '';
		dod = '';
		workspacePath = '';
		runAtLocal = '';
		mode = 'one-shot';
		frequency = 'daily';
		time = '09:00';
		weekday = '1';
		monthDay = '1';
		unsupportedCron = '';
	}

	function openNewForm() {
		resetForm();
		workspacePath = deps.getWorkspacePath()?.trim() ?? '';
		showForm = true;
	}

	function cancelForm() {
		resetForm();
		showForm = false;
	}

	function selectOneShot() {
		mode = 'one-shot';
		unsupportedCron = '';
	}

	function editScheduled(job: ScheduledJobResource) {
		editingId = job.id;
		description = job.description;
		dod = job.definition_of_done ?? '';
		workspacePath = job.workspace_path ?? '';
		unsupportedCron = '';
		if (job.cron_expr) {
			mode = 'recurring';
			const parsed = parseSupportedCron(job.cron_expr);
			if (parsed) {
				time = parsed.time;
				frequency = parsed.frequency;
				if (parsed.frequency === 'weekly') weekday = parsed.weekday;
				if (parsed.frequency === 'monthly') monthDay = parsed.monthDay;
			} else {
				frequency = 'daily';
				time = '09:00';
				unsupportedCron = job.cron_expr;
			}
		} else {
			mode = 'one-shot';
			runAtLocal = millisToLocalDatetime(job.run_at ?? job.next_run_at);
		}
		showForm = true;
	}

	async function handleSaveScheduled() {
		if (!description.trim()) return;
		const cron = mode === 'recurring' ? generatedCron : '';
		const runAt = mode === 'one-shot' ? localDatetimeToMillis(runAtLocal) : undefined;
		if (mode === 'one-shot' && !runAt) {
			deps.setError('Choose when this one-shot schedule should run.');
			return;
		}
		deps.setSaving(true);
		deps.setError('');
		try {
			const scheduleFields: UpdateScheduledJobRequest = {
				description: description.trim(),
				definition_of_done: dod.trim() || undefined,
				workspace_path: workspacePath.trim() || undefined,
				cron_expr: cron || undefined,
				run_at: runAt
			};
			if (editingId) {
				await updateScheduledJob(editingId, scheduleFields);
			} else {
				const body: CreateScheduledJobRequest = {
					description: description.trim(),
					definition_of_done: scheduleFields.definition_of_done,
					workspace_path: scheduleFields.workspace_path,
					cron_expr: scheduleFields.cron_expr,
					run_at: scheduleFields.run_at,
					created_by: 'user',
					source_platform: 'desktop'
				};
				await createScheduledJob(body);
			}
			resetForm();
			showForm = false;
			await loadScheduledJobs({ silent: true });
		} catch (e) {
			deps.setError(e instanceof Error ? e.message : 'Failed to save scheduled job');
		} finally {
			deps.setSaving(false);
		}
	}

	async function handleDeleteScheduled(job: ScheduledJobResource) {
		if (!confirm(`Delete scheduled job "${truncateJobLabel(job.description)}"?`)) return;
		try {
			await deleteScheduledJob(job.id);
			await loadScheduledJobs({ silent: true });
		} catch (e) {
			deps.setError(e instanceof Error ? e.message : 'Failed to delete scheduled job');
		}
	}

	async function handleToggleScheduled(job: ScheduledJobResource) {
		try {
			await updateScheduledJob(job.id, { enabled: !job.enabled });
			await loadScheduledJobs({ silent: true });
		} catch (e) {
			deps.setError(e instanceof Error ? e.message : 'Failed to toggle scheduled job');
		}
	}

	return {
		get scheduledJobs() {
			return scheduledJobs;
		},
		get showForm() {
			return showForm;
		},
		get editingId() {
			return editingId;
		},
		get description() {
			return description;
		},
		set description(value: string) {
			description = value;
		},
		get dod() {
			return dod;
		},
		set dod(value: string) {
			dod = value;
		},
		get workspacePath() {
			return workspacePath;
		},
		set workspacePath(value: string) {
			workspacePath = value;
		},
		get runAtLocal() {
			return runAtLocal;
		},
		set runAtLocal(value: string) {
			runAtLocal = value;
		},
		get mode() {
			return mode;
		},
		set mode(value: ScheduleMode) {
			mode = value;
		},
		get frequency() {
			return frequency;
		},
		set frequency(value: ScheduleFrequency) {
			frequency = value;
		},
		get time() {
			return time;
		},
		set time(value: string) {
			time = value;
		},
		get weekday() {
			return weekday;
		},
		set weekday(value: string) {
			weekday = value;
		},
		get monthDay() {
			return monthDay;
		},
		set monthDay(value: string) {
			monthDay = value;
		},
		get unsupportedCron() {
			return unsupportedCron;
		},
		get generatedCron() {
			return generatedCron;
		},
		get summary() {
			return summary;
		},
		loadScheduledJobs,
		openNewForm,
		cancelForm,
		selectOneShot,
		editScheduled,
		handleSaveScheduled,
		handleDeleteScheduled,
		handleToggleScheduled
	};
}

export type JobsScheduleController = ReturnType<typeof createJobsScheduleController>;
