<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/Composer.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { createSession } from '$lib/client/cometmind';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { chatStore } from '$lib/stores/chat.svelte';

	let bootMessage = $derived(shellStore.bootMessage);

	// Entering the home route is a one-shot reset: no reactive inputs, so this
	// is a lifecycle action, not a reactive effect.
	onMount(() => {
		sessionStore.selectSession(null);
		chatStore.clear();
		shellStore.centerComposer();
	});

	async function onSend(text: string) {
		const selectedModel = modelStore.selected;
		if (!selectedModel) return;
		const session = await createSession({
			workspace_path: shellStore.workspacePath,
			model_id: selectedModel.modelId,
			provider_id: selectedModel.providerId
		});
		sessionStore.appendSession(session);
		sessionStore.queuePendingMessage(session.id, text);
		await goto(`/session/${session.id}`);
	}
</script>

<div class="chat-home hero-layout">
	<div class="empty-region">
		<EmptyChatState />
		{#if bootMessage}
			<p class="boot-error">{bootMessage}</p>
		{/if}
	</div>

	<div class="composer-wrapper centered">
		<Composer {onSend} disabled={connectionState.status !== 'ready'} variant="hero" />
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

	.chat-home.hero-layout .composer-wrapper :global(.composer) {
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

	.boot-error {
		margin: 18px 0 0;
		max-width: 520px;
		font-size: 12px;
		line-height: 1.5;
		color: #b42318;
		text-align: center;
	}

	@media (max-width: 900px) {
		.chat-home.hero-layout {
			gap: 40px;
			padding: 32px 28px;
		}
	}
</style>
