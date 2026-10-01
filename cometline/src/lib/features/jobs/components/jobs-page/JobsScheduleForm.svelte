<script lang="ts">
	import WorkspacePathField from '$lib/components/WorkspacePathField.svelte';
	import type { JobsScheduleController } from '$lib/features/jobs/jobs-page-schedule.svelte';

	let { schedule }: { schedule: JobsScheduleController } = $props();

	function openNativePicker(event: Event & { currentTarget: HTMLInputElement }) {
		try {
			event.currentTarget.showPicker();
		} catch {
			// Unsupported browser, or picker already open.
		}
	}
</script>

<form
	id="schedule-job-form"
	class="schedule-form"
	onsubmit={(e) => {
		e.preventDefault();
		void schedule.handleSaveScheduled();
	}}
>
	<label class="form-field">
		<span>Description</span>
		<textarea
			bind:value={schedule.description}
			rows="4"
			placeholder="What should this job do?"
			required
		></textarea>
	</label>
	<label class="form-field">
		<span>Definition of done</span>
		<textarea bind:value={schedule.dod} rows="4" placeholder="How will you know it's finished?"
		></textarea>
	</label>
	<div class="form-field">
		<span>Workspace path</span>
		<WorkspacePathField bind:value={schedule.workspacePath} />
	</div>
	<div class="schedule-kind" role="group" aria-label="Schedule type">
		<button
			type="button"
			class:active={schedule.mode === 'one-shot'}
			onclick={schedule.selectOneShot}
		>
			One time
		</button>
		<button
			type="button"
			class:active={schedule.mode === 'recurring'}
			onclick={() => (schedule.mode = 'recurring')}
		>
			Recurring
		</button>
	</div>
	<div class="schedule-mode">
		{#if schedule.mode === 'one-shot'}
			<label class="form-field">
				<span>Run at</span>
				<input
					type="datetime-local"
					bind:value={schedule.runAtLocal}
					onclick={openNativePicker}
				/>
				<small>Local time on this device.</small>
			</label>
		{:else}
			<label class="form-field">
				<span>Repeat</span>
				<select bind:value={schedule.frequency}>
					<option value="daily">Every day</option>
					<option value="weekly">Every week</option>
					<option value="monthly">Every month</option>
				</select>
			</label>
			{#if schedule.frequency === 'weekly'}
				<label class="form-field">
					<span>Weekday</span>
					<select bind:value={schedule.weekday}>
						<option value="1">Monday</option>
						<option value="2">Tuesday</option>
						<option value="3">Wednesday</option>
						<option value="4">Thursday</option>
						<option value="5">Friday</option>
						<option value="6">Saturday</option>
						<option value="0">Sunday</option>
					</select>
				</label>
			{/if}
			{#if schedule.frequency === 'monthly'}
				<label class="form-field">
					<span>Day of month</span>
					<select bind:value={schedule.monthDay}>
						{#each Array.from({ length: 28 }, (_, i) => String(i + 1)) as day (day)}
							<option value={day}>{day}</option>
						{/each}
					</select>
					<small>Limited to days 1-28 so every month has the date.</small>
				</label>
			{/if}
			<label class="form-field">
				<span>Time</span>
				<input type="time" bind:value={schedule.time} onclick={openNativePicker} />
			</label>
			<div class="schedule-generated">
				<span>{schedule.summary}</span>
				<code>{schedule.generatedCron}</code>
				{#if schedule.unsupportedCron}
					<small
						>Existing custom cron <code>{schedule.unsupportedCron}</code> is not editable
						with this picker. Saving will replace it with the schedule above.</small
					>
				{/if}
			</div>
		{/if}
	</div>
</form>

<style>
	.schedule-form {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 12px;
		border-radius: 10px;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
	}

	.form-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.form-field input,
	.form-field textarea,
	.form-field select {
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		padding: 7px 9px;
		border: 1px solid var(--border-soft);
		border-radius: 7px;
		background: var(--panel-bg);
		min-height: 34px;
		resize: vertical;
	}

	.form-field input[type='datetime-local'],
	.form-field input[type='time'] {
		cursor: pointer;
	}

	.schedule-form .form-field textarea {
		min-height: 96px;
		max-height: 220px;
		overflow-y: auto;
		line-height: 1.45;
	}

	.form-field small {
		font-size: 10px;
		color: var(--text-muted);
	}

	.schedule-mode {
		display: grid;
		gap: 10px;
	}

	.schedule-kind {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		width: fit-content;
		padding: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.05);
	}

	.schedule-kind button {
		border: none;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		font-weight: 650;
		padding: 5px 10px;
		border-radius: 999px;
		cursor: pointer;
	}

	.schedule-kind button.active {
		background: var(--panel-bg);
		color: var(--text-main);
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
	}

	.schedule-generated {
		display: flex;
		flex-direction: column;
		gap: 4px;
		justify-content: center;
		padding: 8px 10px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--accent) 8%, transparent);
		border: 1px solid color-mix(in srgb, var(--accent) 14%, var(--border-soft));
		font-size: 11px;
		color: var(--text-main);
	}

	.schedule-generated code {
		width: fit-content;
		font-size: 11px;
	}

	.schedule-generated small {
		font-size: 10px;
		line-height: 1.4;
		color: var(--text-muted);
	}
</style>
