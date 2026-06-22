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
	import { FolderOpen } from '@lucide/svelte';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';

	let bootMessage = $derived(shellStore.bootMessage);

	// Entering the home route is a one-shot reset: no reactive inputs, so this
	// is a lifecycle action, not a reactive effect.
	onMount(() => {
		sessionStore.selectSession(null);
		chatStore.detachActiveSession();
		shellStore.clearDraftPanel();
		shellStore.centerComposer();
		shellStore.resetActiveToDefault();
		modelStore.selectDefault();
	});

	function openSettings() {
		shellStore.openSettings();
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
		<HeroComposerFrame>
			<Composer {onSend} disabled={connectionState.status !== 'ready'} variant="hero" />
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
		color: #b42318;
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

	@media (max-width: 900px) {
		.chat-home.hero-layout {
			gap: 40px;
			padding: 32px 28px;
		}
	}
</style>
