<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/composer/Composer.svelte';
	import HeroComposerFrame from '$lib/components/HeroComposerFrame.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { createSession } from '$lib/client/cometmind';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { FolderOpen, X } from '@lucide/svelte';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import { renderUserText } from '$lib/markdown/render';

	let bootMessage = $derived(shellStore.bootMessage);
	let composerRef = $state<{ focus: () => void } | null>(null);
	let composerFocusRequestId = $derived(shellStore.composerFocusRequestId);
	let listJobsMessage = $state<string | null>(null);

	$effect(() => {
		if (!composerFocusRequestId || shellStore.focusedPane !== 'chat') return;
		setTimeout(() => composerRef?.focus(), 0);
	});

	// Entering the home route is a one-shot reset: no reactive inputs, so this
	// is a lifecycle action, not a reactive effect.
	onMount(() => {
		sessionStore.selectSession(null);
		chatStore.detachActiveSession();
		shellStore.clearDraftPanel();
		shellStore.centerComposer();
		shellStore.resetActiveToDefault();
		modelStore.selectDefault();
		listJobsMessage = null;
	});

	function openSettings() {
		shellStore.openSettings();
	}

	function showLocalUserMessage(text: string) {
		listJobsMessage = text;
	}

	function dismissListJobsMessage() {
		listJobsMessage = null;
	}

	async function onSend(payload: ChatTurnPayload | string) {
		const message = typeof payload === 'string' ? { text: payload } : payload;
		const selectedModel = modelStore.selected;
		if (!selectedModel) return;
		const workspace = shellStore.workspacePath;
		const session = await createSession({
			workspace_path: workspace,
			model_id: selectedModel.modelId,
			provider_id: selectedModel.providerId
		});
		sessionStore.appendSession(session);
		sessionStore.queuePendingMessage(
			session.id,
			message.text,
			message.images,
			message.filePaths,
			message.displayText
		);
		shellStore.migrateDraftPanel(session.id);
		await goto(`/session/${session.id}`);
	}
</script>

<div class="chat-home hero-layout">
	<div class="empty-region">
		<EmptyChatState />
		{#if bootMessage}
			<div class="boot-error-wrap">
				<p class="boot-error">{bootMessage}</p>
				<button class="set-workspace-button" onclick={openSettings}>
					<FolderOpen size={14} />
					Set workspace
				</button>
			</div>
		{/if}
	</div>

	<div class="composer-wrapper centered">
		{#if listJobsMessage}
			<div class="list-jobs-panel chat-bubble user" role="status">
				<button
					type="button"
					class="list-jobs-dismiss"
					aria-label="Dismiss"
					onclick={dismissListJobsMessage}
				>
					<X size={14} />
				</button>
				<div class="list-jobs-content">{@html renderUserText(listJobsMessage)}</div>
			</div>
		{/if}
		<HeroComposerFrame>
			<Composer
				bind:this={composerRef}
				{onSend}
				onLocalUserMessage={showLocalUserMessage}
				disabled={connectionState.status !== 'ready'}
				variant="hero"
			/>
		</HeroComposerFrame>
	</div>
</div>

<style>
	.chat-home {
		position: relative;
		flex: 1;
		min-height: 0;
		width: 100%;
		overflow: hidden;
	}

	.chat-home.hero-layout {
		display: grid;
		place-items: center;
		align-content: center;
		gap: 52px;
		padding: 48px;
	}

	.chat-home.hero-layout .composer-wrapper {
		position: relative;
		bottom: auto;
		left: auto;
		transform: none;
		width: 100%;
		padding: 0 var(--chat-gutter);
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		justify-content: center;
	}

	.chat-home.hero-layout .composer-wrapper :global(.hero-composer-frame) {
		width: min(var(--chat-composer-width), 100%);
		max-width: 100%;
	}

	.empty-region {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 0;
	}

	.boot-error-wrap {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		margin-top: 18px;
	}

	.boot-error {
		margin: 0;
		max-width: 520px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--status-error);
		text-align: center;
	}

	.set-workspace-button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
		background: rgba(15, 23, 42, 0.04);
		border: none;
		border-radius: 10px;
		cursor: pointer;
	}

	.set-workspace-button:hover {
		background: rgba(15, 23, 42, 0.08);
	}

	.list-jobs-panel {
		position: relative;
		width: min(var(--chat-composer-width), 100%);
		max-width: 100%;
		padding: 12px 36px 12px 14px;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.list-jobs-dismiss {
		position: absolute;
		top: 8px;
		right: 8px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.list-jobs-dismiss:hover {
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-main);
	}

	.list-jobs-content {
		font-size: 13px;
		line-height: 1.5;
	}

	@media (max-width: 900px) {
		.chat-home.hero-layout {
			gap: 40px;
			padding: 32px 28px;
		}
	}
</style>
