<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import type { InboxMessageResource } from '#lib/client/cometmind.js';
	import InboxLinkStatus from '#lib/features/inbox/components/InboxLinkStatus.svelte';
	import { createInboxDrawerController } from '#lib/features/inbox/inbox-drawer-controller.svelte.js';
	import InboxDrawerHeader from './inbox-drawer/InboxDrawerHeader.svelte';
	import InboxMessageDetail from './inbox-drawer/InboxMessageDetail.svelte';
	import InboxMessageList from './inbox-drawer/InboxMessageList.svelte';

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

	const drawer = createInboxDrawerController({
		getOpen: () => open,
		getMessages: () => messages,
		getBusyId: () => busyId,
		canOpenJob: () => Boolean(onOpenJob),
		canOpenSession: () => Boolean(onOpenSession),
		reply: (id, content) => onReply(id, content),
		dismiss: (id) => onDismiss(id)
	});

	function handleWindowKeydown(event: KeyboardEvent) {
		if (!open || event.key !== 'Escape') return;
		event.preventDefault();
		onClose();
	}
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#if open}
	<InboxLinkStatus {messages} onAvailability={drawer.setLinkAvailability} />
	<div class="inbox-layer" transition:fade={{ duration: 120 }}>
		<button type="button" class="inbox-scrim" aria-label="Close inbox" onclick={onClose}
		></button>
		<div
			class="inbox-modal"
			class:has-selection={drawer.selected !== null}
			role="dialog"
			aria-modal="true"
			aria-label="Inbox"
			transition:scale={{ start: 0.97, duration: 140 }}
		>
			<InboxDrawerHeader count={messages.length} {onClose} />

			{#if error}
				<p class="inbox-error">{error}</p>
			{/if}

			{#if messages.length === 0}
				<div class="inbox-empty">
					<p>No messages waiting for you.</p>
				</div>
			{:else}
				<div class="inbox-body">
					<InboxMessageList
						{messages}
						selectedId={drawer.selected?.id}
						onSelect={drawer.selectMessage}
					/>
					<InboxMessageDetail controller={drawer} {busyId} {onOpenJob} {onOpenSession} />
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
		background: var(--panel-bg, var(--panel-bg));
		border: 1px solid var(--border-soft);
		border-radius: 18px;
		box-shadow: 0 22px 70px rgba(15, 23, 42, 0.18);
		pointer-events: auto;
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
		color: var(--status-error);
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

	@media (max-width: 640px) {
		.inbox-layer {
			padding: 12px;
		}

		.inbox-modal.has-selection .inbox-body {
			grid-template-columns: 1fr;
			grid-template-rows: minmax(0, 36%) minmax(0, 1fr);
		}
	}
</style>
