<script lang="ts">
	import type { SessionPlanResponse, SessionPlanStep } from '$lib/client/cometmind';

	// How long to keep a fully-completed checklist visible (so the last item's
	// checkmark is seen) before auto-fading it out.
	const AUTO_DISMISS_DELAY_MS = 3200;

	let {
		plan,
		loading = false,
		error = '',
		onDismiss
	}: {
		plan: SessionPlanResponse | null;
		loading?: boolean;
		error?: string;
		onDismiss?: () => void;
	} = $props();

	let steps = $derived(plan?.steps ?? []);
	let allCompleted = $derived(steps.length > 0 && steps.every((s) => s.status === 'completed'));
	// Identity of "this particular plan" so a brand-new plan (different
	// session, or the agent started a fresh plan_write, which replaces all
	// step rows with new ids) resets dismissal even if the previous plan had
	// been auto-hidden or closed by the user.
	let planIdentity = $derived(plan ? `${plan.session_id}:${steps[0]?.id ?? ''}` : '');

	let manuallyDismissed = $state(false);
	let autoDismissed = $state(false);
	let lastPlanIdentity = '';
	let dismissTimeout: ReturnType<typeof setTimeout> | undefined;

	$effect(() => {
		if (planIdentity !== lastPlanIdentity) {
			lastPlanIdentity = planIdentity;
			manuallyDismissed = false;
			autoDismissed = false;
		}
	});

	$effect(() => {
		if (dismissTimeout) {
			clearTimeout(dismissTimeout);
			dismissTimeout = undefined;
		}
		if (!allCompleted || autoDismissed || manuallyDismissed) return;
		dismissTimeout = setTimeout(() => {
			autoDismissed = true;
		}, AUTO_DISMISS_DELAY_MS);
		return () => {
			if (dismissTimeout) clearTimeout(dismissTimeout);
		};
	});

	let dismissed = $derived(Boolean(plan?.dismissed) || manuallyDismissed || autoDismissed);
	let hasContent = $derived((steps.length > 0 || Boolean(error)) && !dismissed);

	function statusLabel(status: SessionPlanStep['status']) {
		return status.replace('_', ' ');
	}

	function handleDismiss() {
		manuallyDismissed = true;
		onDismiss?.();
	}
</script>

{#if hasContent}
	<aside class="session-plan" class:completed={allCompleted} aria-label="Session plan">
		<header class="plan-header">
			<div>
				<p class="eyebrow">Current plan</p>
				<h2>Agent checklist</h2>
			</div>
			{#if loading}
				<span class="plan-loading">Refreshing</span>
			{/if}
			<button
				type="button"
				class="plan-close"
				aria-label="Dismiss plan"
				onclick={handleDismiss}
			>
				&times;
			</button>
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
		opacity: 1;
		transition:
			opacity 0.4s ease,
			transform 0.4s ease;
	}

	.session-plan.completed {
		animation: plan-complete-pulse 0.5s ease;
	}

	@keyframes plan-complete-pulse {
		0% {
			transform: scale(1);
		}
		40% {
			transform: scale(1.015);
		}
		100% {
			transform: scale(1);
		}
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

	.plan-close {
		flex: none;
		display: grid;
		place-items: center;
		width: 24px;
		height: 24px;
		margin: -4px -4px 0 0;
		border: none;
		border-radius: 999px;
		background: transparent;
		color: var(--text-muted);
		font-size: 16px;
		line-height: 1;
		cursor: pointer;
		transition:
			background 0.15s ease,
			color 0.15s ease;
	}

	.plan-close:hover {
		background: color-mix(in srgb, var(--text-main) 8%, transparent);
		color: var(--text-main);
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
