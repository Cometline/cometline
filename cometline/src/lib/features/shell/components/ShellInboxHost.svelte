<script lang="ts">
	import type InboxDrawerComponent from '#lib/features/inbox/components/InboxDrawer.svelte';
	import { getSession } from '#lib/client/cometmind.js';
	import { navigateToSession } from '#lib/actions/navigate-to-session.js';
	import { gotoJob } from '#lib/routes/job-route.js';
	import { inboxStore } from '#lib/stores/inbox.svelte.js';

	let {
		drawer: Drawer = null,
		loadFailed,
		onRetry
	}: {
		drawer?: typeof InboxDrawerComponent | null;
		loadFailed: boolean;
		onRetry: () => void;
	} = $props();
</script>

{#if Drawer}
	<Drawer
		open={inboxStore.drawerOpen}
		messages={inboxStore.messages}
		busyId={inboxStore.busyId}
		error={inboxStore.error}
		onClose={() => inboxStore.closeDrawer()}
		onReply={(id: string, content: string) => inboxStore.reply(id, content)}
		onDismiss={(id: string) => inboxStore.dismiss(id)}
		onOpenJob={(jobId: string) => {
			inboxStore.closeDrawer();
			void gotoJob(jobId);
		}}
		onOpenSession={(sessionId: string) => {
			inboxStore.closeDrawer();
			void getSession(sessionId)
				.then((session) => navigateToSession(session))
				.catch(() => {
					/* session may already be purged */
				});
		}}
	/>
{:else if inboxStore.drawerOpen}
	<div class="inbox-loading-layer" role="dialog" aria-modal="true" aria-label="Inbox">
		<button
			type="button"
			class="inbox-loading-scrim"
			aria-label="Close inbox"
			onclick={() => inboxStore.closeDrawer()}
		></button>
		<div class="inbox-loading-card" aria-busy={!loadFailed}>
			{#if loadFailed}
				<p>Inbox failed to load.</p>
				<div class="inbox-loading-actions">
					<button type="button" onclick={() => inboxStore.closeDrawer()}>Close</button>
					<button type="button" class="primary" onclick={onRetry}>Retry</button>
				</div>
			{:else}
				<p>Loading inbox…</p>
			{/if}
		</div>
	</div>
{/if}

<style>
	.inbox-loading-layer {
		position: fixed;
		inset: 0;
		z-index: 75;
		display: grid;
		place-items: center;
		padding: 24px;
	}

	.inbox-loading-scrim {
		position: fixed;
		inset: 0;
		border: 0;
		background: rgba(17, 24, 39, 0.28);
		backdrop-filter: blur(10px);
	}

	.inbox-loading-card {
		position: relative;
		display: grid;
		min-width: 260px;
		gap: 16px;
		place-items: center;
		padding: 28px;
		border: 1px solid var(--border-soft);
		border-radius: 18px;
		background: var(--panel-bg);
		box-shadow: 0 22px 70px rgba(15, 23, 42, 0.18);
		color: var(--text-muted);
	}

	.inbox-loading-card p {
		margin: 0;
	}

	.inbox-loading-actions {
		display: flex;
		gap: 8px;
	}

	.inbox-loading-actions button {
		padding: 7px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg);
		color: var(--text-main);
		font: inherit;
		cursor: pointer;
	}

	.inbox-loading-actions button.primary {
		border-color: var(--accent);
		background: var(--accent);
		color: var(--panel-bg);
	}
</style>
