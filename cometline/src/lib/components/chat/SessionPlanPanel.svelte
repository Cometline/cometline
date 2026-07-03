<script lang="ts">
	import type { SessionPlanResponse, SessionPlanStep } from '$lib/client/cometmind';

	let {
		plan,
		loading = false,
		error = ''
	}: { plan: SessionPlanResponse | null; loading?: boolean; error?: string } = $props();

	let steps = $derived(plan?.steps ?? []);
	let hasContent = $derived(steps.length > 0 || Boolean(error));

	function statusLabel(status: SessionPlanStep['status']) {
		return status.replace('_', ' ');
	}
</script>

{#if hasContent}
	<aside class="session-plan" aria-label="Session plan">
		<header class="plan-header">
			<div>
				<p class="eyebrow">Current plan</p>
				<h2>Agent checklist</h2>
			</div>
			{#if loading}
				<span class="plan-loading">Refreshing</span>
			{/if}
		</header>

		{#if error}
			<p class="plan-error">{error}</p>
		{:else}
			<ol class="plan-steps">
				{#each steps as step (step.id)}
					<li class="plan-step" class:completed={step.status === 'completed'}>
						<span class="step-marker" aria-hidden="true">
							{#if step.status === 'completed'}
								✓
							{:else}
								{step.step_index + 1}
							{/if}
						</span>
						<div class="step-body">
							<div class="step-line">
								<span class="step-description">{step.description}</span>
								<span class="status-chip" data-status={step.status}
									>{statusLabel(step.status)}</span
								>
							</div>
							{#if step.status === 'blocked' && step.blocker_reason}
								<p class="blocker">{step.blocker_reason}</p>
							{/if}
						</div>
					</li>
				{/each}
			</ol>
		{/if}
	</aside>
{/if}

<style>
	.session-plan {
		width: min(520px, calc(100vw - 32px));
		max-height: min(42vh, 360px);
		overflow: auto;
		padding: 14px;
		border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
		border-radius: 22px;
		background:
			linear-gradient(
				135deg,
				color-mix(in srgb, var(--panel-bg) 94%, var(--hero-composer-glow-color) 6%),
				color-mix(in srgb, var(--panel-bg) 88%, transparent)
			),
			var(--panel-bg);
		box-shadow: 0 18px 42px color-mix(in srgb, var(--shadow-color) 16%, transparent);
		backdrop-filter: blur(18px);
	}

	.plan-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 12px;
	}

	.eyebrow {
		margin: 0 0 3px;
		font-size: 10px;
		font-weight: 750;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--text-muted);
	}

	h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 760;
		color: var(--text-main);
	}

	.plan-loading {
		flex: none;
		padding: 4px 8px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--hero-composer-glow-color) 12%, transparent);
		color: var(--text-muted);
		font-size: 11px;
	}

	.plan-error {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--status-error);
	}

	.plan-steps {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.plan-step {
		display: grid;
		grid-template-columns: 24px minmax(0, 1fr);
		gap: 9px;
		align-items: flex-start;
	}

	.step-marker {
		display: grid;
		place-items: center;
		width: 22px;
		height: 22px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--text-main) 7%, transparent);
		color: var(--text-muted);
		font-size: 11px;
		font-weight: 720;
	}

	.plan-step.completed .step-marker {
		background: color-mix(in srgb, var(--status-success) 16%, transparent);
		color: var(--status-success);
	}

	.step-body {
		min-width: 0;
	}

	.step-line {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 10px;
	}

	.step-description {
		min-width: 0;
		color: var(--text-main);
		font-size: 12px;
		line-height: 1.45;
	}

	.plan-step.completed .step-description {
		color: var(--text-muted);
		text-decoration: line-through;
		text-decoration-thickness: 1px;
	}

	.status-chip {
		flex: none;
		padding: 3px 7px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--text-main) 7%, transparent);
		color: var(--text-muted);
		font-size: 10px;
		font-weight: 720;
		text-transform: capitalize;
	}

	.status-chip[data-status='in_progress'] {
		background: color-mix(in srgb, var(--hero-composer-glow-color) 18%, transparent);
		color: var(--text-main);
	}

	.status-chip[data-status='completed'] {
		background: color-mix(in srgb, var(--status-success) 14%, transparent);
		color: var(--status-success);
	}

	.status-chip[data-status='blocked'] {
		background: color-mix(in srgb, var(--status-error) 12%, transparent);
		color: var(--status-error);
	}

	.blocker {
		margin: 5px 0 0;
		padding-left: 9px;
		border-left: 2px solid color-mix(in srgb, var(--status-error) 34%, transparent);
		color: var(--text-muted);
		font-size: 11px;
		line-height: 1.4;
	}

	@media (max-width: 720px) {
		.session-plan {
			width: calc(100vw - 24px);
			max-height: 32vh;
			padding: 12px;
		}

		.step-line {
			flex-direction: column;
			gap: 5px;
		}
	}
</style>
