<script lang="ts">
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

	$effect(() => {
		sessionStore.selectSession(null);
		chatStore.clear();
		shellStore.centerComposer();
	});

	async function onSend(text: string) {
		const session = await createSession({
			workspace_path: shellStore.workspacePath,
			model_id: modelStore.selected.model_id,
			provider_id: modelStore.selected.provider_id
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
		<Composer
			onSend={onSend}
			disabled={connectionState.status !== 'ready'}
			variant="hero"
		/>
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
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 36px;
		padding: 48px;
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

	.composer-wrapper {
		position: absolute;
		left: 50%;
		transform: translateX(-50%);
		width: min(var(--composer-width), calc(100% - 64px));
		z-index: 10;
		bottom: 40px;
	}

	.composer-wrapper.centered {
		bottom: 50%;
		transform: translateX(-50%) translateY(168px);
		width: min(700px, calc(100% - 64px));
	}

	@media (max-width: 900px) {
		.composer-wrapper {
			bottom: 24px;
			width: calc(100% - 40px);
		}

		.chat-home.hero-layout {
			gap: 28px;
			padding: 32px 28px;
		}
	}
</style>
