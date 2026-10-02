<script lang="ts">
	import type { InboxMessageResource } from '#lib/client/cometmind.js';
	import type { LinkAvailability } from '#lib/features/inbox/link-availability.js';

	let {
		message,
		jobLinkStatus,
		sessionLinkStatus,
		showJobLink,
		showSessionLink,
		onOpenJob,
		onOpenSession
	}: {
		message: InboxMessageResource;
		jobLinkStatus: LinkAvailability | null;
		sessionLinkStatus: LinkAvailability | null;
		showJobLink: boolean;
		showSessionLink: boolean;
		onOpenJob?: (jobId: string) => void;
		onOpenSession?: (sessionId: string) => void;
	} = $props();
</script>

<div class="detail-links">
	{#if showJobLink && message.job_id}
		{#if jobLinkStatus === 'available' && onOpenJob}
			<button type="button" class="link-btn" onclick={() => onOpenJob(message.job_id!)}>
				Open job
			</button>
		{:else if jobLinkStatus === 'missing'}
			<span class="link-dead" title="This link is no longer available">
				Job unavailable
			</span>
		{/if}
	{/if}
	{#if showSessionLink && message.session_id}
		{#if sessionLinkStatus === 'available' && onOpenSession}
			<button
				type="button"
				class="link-btn"
				onclick={() => onOpenSession(message.session_id!)}
			>
				Open session
			</button>
		{:else if sessionLinkStatus === 'missing'}
			<span class="link-dead" title="This link is no longer available">
				Session unavailable
			</span>
		{/if}
	{/if}
</div>

<style>
	.detail-links {
		display: flex;
		flex-wrap: wrap;
		gap: 10px;
	}

	.link-btn {
		border: none;
		background: transparent;
		padding: 0;
		font-size: 12px;
		font-weight: 500;
		color: var(--accent);
		cursor: pointer;
	}

	.link-btn:hover {
		text-decoration: underline;
	}

	.link-dead {
		font-size: 12px;
		font-weight: 500;
		color: var(--text-muted, var(--text-soft));
		cursor: default;
		user-select: none;
	}
</style>
