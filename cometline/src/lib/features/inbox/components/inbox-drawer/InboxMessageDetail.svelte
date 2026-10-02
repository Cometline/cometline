<script lang="ts">
	import AssistantMarkdown from '#lib/components/AssistantMarkdown.svelte';
	import type { InboxDrawerController } from '#lib/features/inbox/inbox-drawer-controller.svelte.js';
	import { formatRelativeTime } from '#lib/features/inbox/inbox-drawer-format.js';
	import InboxDetailLinks from './InboxDetailLinks.svelte';

	let {
		controller,
		busyId,
		onOpenJob,
		onOpenSession
	}: {
		controller: InboxDrawerController;
		busyId: string | null;
		onOpenJob?: (jobId: string) => void;
		onOpenSession?: (sessionId: string) => void;
	} = $props();

	let replyEl = $state<HTMLTextAreaElement | null>(null);

	$effect(() => {
		const unsubscribe = window.electronAPI?.onCommandEnter?.((signal) => {
			if (signal.purpose !== 'submit') return;
			if (document.activeElement !== replyEl) return;
			void controller.submitReply();
		});
		return () => unsubscribe?.();
	});
</script>

{#if controller.selected}
	{@const selected = controller.selected}
	<div class="inbox-detail">
		<p class="detail-title">{selected.title}</p>
		<p class="detail-meta">{formatRelativeTime(selected.created_at)}</p>
		<div class="detail-body">
			<AssistantMarkdown source={selected.body} />
		</div>
		{#if controller.showDetailLinks}
			<InboxDetailLinks
				message={selected}
				jobLinkStatus={controller.jobLinkStatus}
				sessionLinkStatus={controller.sessionLinkStatus}
				showJobLink={controller.showJobLink}
				showSessionLink={controller.showSessionLink}
				{onOpenJob}
				{onOpenSession}
			/>
		{/if}
		<textarea
			bind:this={replyEl}
			class="reply-input"
			rows="4"
			placeholder="Reply (saved for later; message leaves the inbox)"
			bind:value={() => controller.activeReply, controller.setActiveReply}
			disabled={busyId === selected.id}
			onkeydown={controller.handleReplyKeydown}
		></textarea>
		<div class="detail-actions">
			<button
				type="button"
				class="secondary"
				disabled={busyId === selected.id}
				onclick={() => void controller.dismissSelected()}
			>
				Dismiss
			</button>
			<button
				type="button"
				class="primary"
				disabled={busyId === selected.id || !controller.activeReply.trim()}
				onclick={() => void controller.submitReply()}
			>
				Send reply
			</button>
		</div>
	</div>
{:else}
	<div class="inbox-detail inbox-detail-empty">
		<p>Select a message to read and reply.</p>
	</div>
{/if}

<style>
	.inbox-detail {
		min-height: 0;
		overflow-y: auto;
		padding: 16px 18px 18px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.inbox-detail-empty {
		align-items: center;
		justify-content: center;
		color: var(--text-soft, var(--text-muted));
		font-size: 13px;
		text-align: center;
	}

	.inbox-detail-empty p {
		margin: 0;
	}

	.detail-title {
		margin: 0;
		font-size: 15px;
		font-weight: 650;
		color: var(--text-main);
	}

	.detail-meta {
		margin: -4px 0 0;
		font-size: 12px;
		color: var(--text-muted, var(--text-soft));
	}

	.detail-body {
		margin: 0;
		font-size: 13px;
		line-height: 1.55;
		color: var(--text-main);
		flex: 1;
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.detail-body :global(p) {
		margin: 0 0 0.65em;
	}

	.detail-body :global(p:last-child) {
		margin-bottom: 0;
	}

	.detail-body :global(a) {
		color: var(--accent);
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	.detail-body :global(ul),
	.detail-body :global(ol) {
		margin: 0 0 0.65em;
		padding-left: 1.25em;
	}

	.detail-body :global(pre),
	.detail-body :global(code) {
		overflow-x: auto;
		max-width: 100%;
	}

	.reply-input {
		width: 100%;
		resize: vertical;
		min-height: 88px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 10px 12px;
		font: inherit;
		font-size: 13px;
		line-height: 1.4;
		background: color-mix(in srgb, var(--app-bg, var(--app-bg)) 88%, var(--panel-bg));
		color: var(--text-main);
		box-sizing: border-box;
	}

	.reply-input:focus {
		outline: 2px solid color-mix(in srgb, var(--accent, var(--text-main)) 35%, transparent);
		outline-offset: 1px;
	}

	.detail-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: auto;
	}

	.detail-actions button {
		border-radius: 9px;
		border: 1px solid var(--border-soft);
		padding: 8px 12px;
		font-size: 13px;
		cursor: pointer;
	}

	.detail-actions button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.detail-actions .secondary {
		background: transparent;
		color: var(--text-muted, var(--text-soft));
	}

	.detail-actions .secondary:hover:not(:disabled) {
		background: color-mix(in srgb, var(--text-main) 5%, transparent);
		color: var(--text-main);
	}

	.detail-actions .primary {
		background: var(--accent, var(--text-main));
		border-color: var(--accent, var(--text-main));
		color: var(--panel-bg);
	}

	.detail-actions .primary:hover:not(:disabled) {
		filter: brightness(1.05);
	}
</style>
