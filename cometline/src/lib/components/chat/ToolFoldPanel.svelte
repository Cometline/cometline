<script lang="ts">
	import { slide } from 'svelte/transition';
	import {
		Terminal,
		ChevronDown,
		LoaderCircle,
		TriangleAlert,
		CircleCheck
	} from '@lucide/svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import JobProposeCard from '$lib/components/chat/JobProposeCard.svelte';
	import { parseJobProposal } from '$lib/jobs/parse-job-proposal';
	import {
		dismissJobProposal,
		getJobProposalDismissal,
		isJobProposalDismissed,
		jobProposalDismissalSummary,
		type JobProposalDismissAction
	} from '$lib/jobs/job-proposal-dismissals';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import type { JobResource } from '$lib/client/cometmind';

	const FOLD_IN = { duration: 180 };

	let {
		item,
		label,
		expanded,
		onToggle,
		nested = false,
		contentOnly = false,
		sessionId = '',
		onNotifyAgent,
		onStartJob
	}: {
		item: Extract<ChatItem, { type: 'tool' }>;
		label: string;
		expanded: boolean;
		onToggle: () => void;
		nested?: boolean;
		contentOnly?: boolean;
		sessionId?: string;
		onNotifyAgent?: (payload: ChatTurnPayload) => void | Promise<void>;
		onStartJob?: (job: JobResource) => void | Promise<void>;
	} = $props();

	const isProposeJob = $derived(item.toolName === 'propose_job');
	const jobProposal = $derived(
		isProposeJob && !item.pending && !item.error
			? parseJobProposal(item.input, item.output)
			: null
	);

	let dismissRevision = $state(0);
	const proposalDismissed = $derived.by(() => {
		dismissRevision;
		return Boolean(
			jobProposal && sessionId && isJobProposalDismissed(sessionId, jobProposal)
		);
	});
	const proposalDismissal = $derived.by(() => {
		dismissRevision;
		return jobProposal && sessionId
			? getJobProposalDismissal(sessionId, jobProposal)
			: null;
	});

	function handleProposalDismiss(action: JobProposalDismissAction, jobId?: string) {
		if (!jobProposal || !sessionId) return;
		dismissJobProposal(sessionId, jobProposal, { action, jobId });
		dismissRevision++;
		if (expanded) onToggle();
	}

	function formatToolInput(input: unknown) {
		if (input == null) return '';
		if (typeof input === 'string') return input.trim();
		try {
			return JSON.stringify(input, null, 2);
		} catch {
			return String(input);
		}
	}
</script>

<div class="fold-panel tool-fold-panel" class:error={!!item.error} class:nested class:content-only={contentOnly}>
	{#if !contentOnly}
		<button
			type="button"
			class="fold-toggle tool-fold-toggle"
			aria-expanded={expanded}
			onclick={onToggle}
		>
			<Terminal size={13} />
			<span>{label}</span>
			{#if item.pending}
				<LoaderCircle size={12} class="spin" />
			{:else if item.error}
				<TriangleAlert size={12} />
			{:else}
				<CircleCheck size={12} />
			{/if}
			<ChevronDown size={13} class={expanded ? 'expanded' : ''} />
		</button>
	{/if}
	{#if expanded}
		<div class="fold-body tool-output-body" transition:slide={FOLD_IN}>
			{#if jobProposal && sessionId && !proposalDismissed}
				<JobProposeCard
					proposal={jobProposal}
					{sessionId}
					{onNotifyAgent}
					{onStartJob}
					onDismiss={handleProposalDismiss}
				/>
			{:else if jobProposal && proposalDismissal}
				<p class="proposal-dismissed">{jobProposalDismissalSummary(proposalDismissal)}</p>
			{:else if formatToolInput(item.input)}
				<pre class="tool-input-text scrollbar-none">{formatToolInput(item.input)}</pre>
			{/if}
			{#if item.error}
				<pre class="tool-error-text scrollbar-none">{item.error}</pre>
			{:else if !jobProposal && item.output}
				<pre class="scrollbar-none">{item.output}</pre>
			{/if}
			{#if item.pending && !item.output && !item.error}
				<pre class="scrollbar-none">Running…</pre>
			{/if}
		</div>
	{/if}
</div>

<style>
	/* Base .fold-panel / .fold-toggle / .fold-body styles live in
	   src/lib/styles/fold-panel.css. Only component-specific overrides here. */
	.fold-panel.nested {
		align-self: stretch;
	}

	.fold-panel.nested .fold-toggle {
		align-self: stretch;
	}

	.fold-panel.content-only .tool-output-body {
		margin-top: 0;
	}

	.tool-fold-panel.error .tool-fold-toggle {
		border-color: rgba(239, 68, 68, 0.35);
		color: #b91c1c;
	}

	.tool-input-text {
		margin: 0 0 8px;
		font-size: 11px;
		color: var(--text-muted);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.tool-output-body {
		margin-top: 8px;
		border: 1px solid var(--border-soft);
		background: rgba(15, 23, 42, 0.03);
		border-radius: 10px;
		padding: 8px 10px;
		color: var(--text-muted);
	}

	.tool-output-body pre {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		font-family: inherit;
		white-space: pre-wrap;
		word-break: normal;
		overflow-wrap: break-word;
		max-height: 220px;
		overflow: auto;
	}

	.tool-output-body pre + pre {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	.tool-error-text {
		color: #b42318;
	}

	.proposal-dismissed {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
