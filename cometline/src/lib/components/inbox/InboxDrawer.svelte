<script lang="ts">
	import { Bell, X } from '@lucide/svelte';
	import { fade, scale } from 'svelte/transition';
	import type { InboxMessageResource } from '$lib/client/cometmind';
	import {
		jobLinkKey,
		resolveInboxLinkAvailability,
		sessionLinkKey,
		type LinkAvailabilityMap
	} from '$lib/inbox/link-availability';

	let {
		open = false,
		messages = [],
		busyId = null,
		error = null,
		onClose,
		onReply,
		onDismiss,
		onOpenJob,
		onOpenSession
	}: {
		open?: boolean;
		messages?: InboxMessageResource[];
		busyId?: string | null;
		error?: string | null;
		onClose: () => void;
		onReply: (id: string, content: string) => void | Promise<void>;
		onDismiss: (id: string) => void | Promise<void>;
		onOpenJob?: (jobId: string) => void;
		onOpenSession?: (sessionId: string) => void;
	} = $props();

	let selectedId = $state<string | null>(null);
	let replyDraft = $state('');
	let replyForId = $state<string | null>(null);
	let linkAvailability: LinkAvailabilityMap = $state({});

	/** Prefer the user's pick; otherwise auto-select the first open message. */
	const selected = $derived.by(() => {
		if (!open || messages.length === 0) return null;
		if (selectedId) {
			const match = messages.find((m) => m.id === selectedId);
			if (match) return match;
		}
		return messages[0] ?? null;
	});

	const activeReply = $derived(
		selected && replyForId === selected.id ? replyDraft : ''
	);

	const jobLinkStatus = $derived.by(() => {
		const id = selected?.job_id?.trim();
		if (!id) return null;
		return linkAvailability[jobLinkKey(id)] ?? 'unknown';
	});

	const sessionLinkStatus = $derived.by(() => {
		const id = selected?.session_id?.trim();
		if (!id) return null;
		return linkAvailability[sessionLinkKey(id)] ?? 'unknown';
	});

	const showJobLink = $derived(
		jobLinkStatus === 'missing' || (jobLinkStatus === 'available' && !!onOpenJob)
	);

	const showSessionLink = $derived(
		sessionLinkStatus === 'missing' ||
			(sessionLinkStatus === 'available' && !!onOpenSession)
	);

	const showDetailLinks = $derived(showJobLink || showSessionLink);

	$effect(() => {
		if (!open) {
			linkAvailability = {};
			return;
		}

		const currentMessages = messages;
		const controller = new AbortController();

		void resolveInboxLinkAvailability(currentMessages, {
			signal: controller.signal
		}).then((result) => {
			if (controller.signal.aborted) return;
			linkAvailability = result;
		});

		return () => {
			controller.abort();
		};
	});

	function formatRelativeTime(ms: number): string {
		const delta = Date.now() - ms;
		const minutes = Math.floor(delta / 60_000);
		if (minutes < 1) return 'just now';
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function previewSnippet(body: string, max = 96): string {
		const compact = body.replace(/\s+/g, ' ').trim();
		if (compact.length <= max) return compact;
		return `${compact.slice(0, max - 1)}…`;
	}

	function selectMessage(id: string) {
		selectedId = id;
		replyDraft = '';
		replyForId = id;
	}

	function setActiveReply(value: string) {
		if (!selected) return;
		replyForId = selected.id;
		replyDraft = value;
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (!open || event.key !== 'Escape') return;
		event.preventDefault();
		onClose();
	}

	async function submitReply() {
		if (!selected || !activeReply.trim() || busyId) return;
		const id = selected.id;
		const content = activeReply.trim();
		await onReply(id, content);
		replyDraft = '';
		replyForId = null;
		selectedId = null;
	}

	async function dismissSelected() {
		if (!selected || busyId) return;
		const id = selected.id;
		replyDraft = '';
		replyForId = null;
		selectedId = null;
		await onDismiss(id);
	}
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#if open}
	<div class="inbox-layer" transition:fade={{ duration: 120 }}>
		<button type="button" class="inbox-scrim" aria-label="Close inbox" onclick={onClose}></button>
		<div
			class="inbox-modal"
			class:has-selection={selected !== null}
			class:is-empty={messages.length === 0}
			role="dialog"
			aria-modal="true"
			aria-label="Inbox"
			transition:scale={{ start: 0.97, duration: 140 }}
		>
			<header class="inbox-header">
				<div class="inbox-title">
					<span class="title-mark" aria-hidden="true">
						<Bell size={16} stroke-width={1.8} />
					</span>
					<div>
						<h2>Inbox</h2>
						{#if messages.length > 0}
							<p>
								{`${messages.length} open message${messages.length === 1 ? '' : 's'}`}
							</p>
						{/if}
					</div>
				</div>
				<button type="button" class="icon-btn" aria-label="Close" onclick={onClose}>
					<X size={16} stroke-width={1.8} />
				</button>
			</header>

			{#if error}
				<p class="inbox-error">{error}</p>
			{/if}

			{#if messages.length === 0}
				<div class="inbox-empty">
					<p>No messages waiting for you.</p>
				</div>
			{:else}
				<div class="inbox-body">
					<ul class="inbox-list" aria-label="Inbox messages">
						{#each messages as message (message.id)}
							<li>
								<button
									type="button"
									class="inbox-row"
									class:selected={selected?.id === message.id}
									onclick={() => selectMessage(message.id)}
								>
									<div class="row-top">
										<span class="row-title">{message.title}</span>
										<span class="row-time">{formatRelativeTime(message.created_at)}</span>
									</div>
									<span class="row-preview">{previewSnippet(message.body)}</span>
								</button>
							</li>
						{/each}
					</ul>

					{#if selected}
						<div class="inbox-detail">
							<p class="detail-title">{selected.title}</p>
							<p class="detail-meta">{formatRelativeTime(selected.created_at)}</p>
							<p class="detail-body">{selected.body}</p>
							{#if showDetailLinks}
								<div class="detail-links">
									{#if showJobLink && selected.job_id}
										{#if jobLinkStatus === 'available' && onOpenJob}
											<button
												type="button"
												class="link-btn"
												onclick={() => onOpenJob(selected.job_id!)}
											>
												Open job
											</button>
										{:else if jobLinkStatus === 'missing'}
											<span
												class="link-dead"
												title="This link is no longer available"
											>
												Job unavailable
											</span>
										{/if}
									{/if}
									{#if showSessionLink && selected.session_id}
										{#if sessionLinkStatus === 'available' && onOpenSession}
											<button
												type="button"
												class="link-btn"
												onclick={() => onOpenSession(selected.session_id!)}
											>
												Open session
											</button>
										{:else if sessionLinkStatus === 'missing'}
											<span
												class="link-dead"
												title="This link is no longer available"
											>
												Session unavailable
											</span>
										{/if}
									{/if}
								</div>
							{/if}
							<textarea
								class="reply-input"
								rows="4"
								placeholder="Reply (saved for later; message leaves the inbox)"
								bind:value={() => activeReply, setActiveReply}
								disabled={busyId === selected.id}
							></textarea>
							<div class="detail-actions">
								<button
									type="button"
									class="secondary"
									disabled={busyId === selected.id}
									onclick={() => void dismissSelected()}
								>
									Dismiss
								</button>
								<button
									type="button"
									class="primary"
									disabled={busyId === selected.id || !activeReply.trim()}
									onclick={() => void submitReply()}
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
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.inbox-layer {
		position: fixed;
		inset: 0;
		z-index: 75;
		display: grid;
		place-items: center;
		padding: 24px;
		pointer-events: none;
	}

	.inbox-scrim {
		position: fixed;
		inset: 0;
		border: none;
		background: rgba(17, 24, 39, 0.28);
		backdrop-filter: blur(10px);
		pointer-events: auto;
		cursor: default;
	}

	.inbox-modal {
		position: relative;
		z-index: 1;
		isolation: isolate;
		display: flex;
		flex-direction: column;
		width: min(820px, 94vw);
		height: min(760px, 88vh);
		max-height: min(760px, 88vh);
		overflow: hidden;
		/* Must be opaque — --bg-elevated/--bg-main are not theme tokens. */
		background: var(--panel-bg, #ffffff);
		border: 1px solid var(--border-soft);
		border-radius: 18px;
		box-shadow: 0 22px 70px rgba(15, 23, 42, 0.18);
		pointer-events: auto;
	}

	.inbox-modal.is-empty {
		height: auto;
		max-height: min(360px, 70vh);
		min-height: 240px;
	}

	.inbox-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		padding: 16px 18px 14px;
		border-bottom: 1px solid var(--border-soft);
		flex-shrink: 0;
	}

	.inbox-title {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		min-width: 0;
	}

	.title-mark {
		display: grid;
		place-items: center;
		width: 32px;
		height: 32px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--accent, var(--text-main)) 12%, transparent);
		color: var(--accent, var(--text-main));
		flex-shrink: 0;
	}

	.inbox-title h2 {
		margin: 0;
		font-size: 16px;
		font-weight: 650;
		line-height: 1.25;
		color: var(--text-main);
	}

	.inbox-title p {
		margin: 2px 0 0;
		font-size: 12px;
		color: var(--text-muted, var(--text-soft));
	}

	.icon-btn {
		width: 30px;
		height: 30px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-muted, var(--text-soft));
		display: grid;
		place-items: center;
		cursor: pointer;
		flex-shrink: 0;
	}

	.icon-btn:hover {
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
		color: var(--text-main);
	}

	.inbox-empty {
		flex: 1;
		display: grid;
		place-items: center;
		margin: 0;
		padding: 36px 24px 40px;
		text-align: center;
	}

	.inbox-empty p {
		margin: 0;
		font-size: 13px;
		color: var(--text-soft, var(--text-muted));
	}

	.inbox-error {
		margin: 0;
		padding: 12px 20px 0;
		font-size: 13px;
		color: #b42318;
		text-align: center;
	}

	.inbox-body {
		flex: 1;
		min-height: 0;
		display: grid;
		grid-template-columns: 1fr;
		grid-template-rows: minmax(0, 38%) minmax(0, 1fr);
	}

	.inbox-modal.has-selection .inbox-body {
		grid-template-columns: minmax(0, 42%) minmax(0, 1fr);
		grid-template-rows: 1fr;
	}

	.inbox-list {
		list-style: none;
		margin: 0;
		padding: 8px;
		overflow-y: auto;
		min-height: 0;
		border-bottom: 1px solid var(--border-soft);
	}

	.inbox-modal.has-selection .inbox-list {
		border-bottom: none;
		border-right: 1px solid var(--border-soft);
	}

	.inbox-row {
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 10px 12px;
		border: none;
		border-radius: 10px;
		background: transparent;
		text-align: left;
		cursor: pointer;
		color: var(--text-main);
	}

	.inbox-row:hover,
	.inbox-row.selected {
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
	}

	.row-top {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 8px;
	}

	.row-title {
		font-size: 13px;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.row-time {
		flex-shrink: 0;
		font-size: 11px;
		color: var(--text-muted, var(--text-soft));
	}

	.row-preview {
		font-size: 12px;
		line-height: 1.35;
		color: var(--text-soft, var(--text-muted));
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

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
		line-height: 1.5;
		color: var(--text-main);
		white-space: pre-wrap;
		flex: 1;
	}

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
		background: color-mix(in srgb, var(--app-bg, #fbfbfa) 88%, #ffffff);
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
		color: #ffffff;
	}

	.detail-actions .primary:hover:not(:disabled) {
		filter: brightness(1.05);
	}

	@media (max-width: 640px) {
		.inbox-layer {
			padding: 12px;
		}

		.inbox-modal.has-selection .inbox-body {
			grid-template-columns: 1fr;
			grid-template-rows: minmax(0, 36%) minmax(0, 1fr);
		}

		.inbox-modal.has-selection .inbox-list {
			border-right: none;
			border-bottom: 1px solid var(--border-soft);
		}
	}
</style>
