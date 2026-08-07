<script lang="ts">
	import { onMount } from 'svelte';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/composer/Composer.svelte';
	import HeroComposerFrame from '$lib/components/HeroComposerFrame.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { openSettings } from '$lib/actions/open-settings';
	import { bootstrapHomeSession } from '$lib/actions/bootstrap-home-session';
	import { FolderOpen } from '@lucide/svelte';

	let composerRef = $state<{ focus: () => void } | null>(null);
	let composerFocusRequest = $derived(shellStore.composerFocusRequest);
	let bootMessage = $derived(shellStore.bootMessage);
	let bootstrapError = $state<string | null>(null);
	let bootstrapping = $state(false);

	$effect(() => {
		if (
			!composerFocusRequest.id ||
			composerFocusRequest.sessionId !== null ||
			shellStore.focusedPane !== 'chat'
		)
			return;
		composerRef?.focus();
	});

	onMount(() => {
		sessionStore.selectSession(null);
		chatStore.detachActiveSession();
		shellStore.centerComposer();
		shellStore.resetActiveToDefault();
		modelStore.selectDefault();

		let cancelled = false;
		let attempted = false;

		const tryBootstrap = () => {
			if (cancelled || attempted || bootstrapping) return;
			if (connectionState.status !== 'ready') return;
			const workspace = shellStore.defaultWorkspacePath || shellStore.workspacePath;
			if (!workspace || workspace === '/') return;

			attempted = true;
			bootstrapping = true;
			bootstrapError = null;
			void bootstrapHomeSession()
				.then((started) => {
					if (cancelled) return;
					if (!started) {
						attempted = false;
						bootstrapping = false;
					}
				})
				.catch((err) => {
					if (cancelled) return;
					bootstrapping = false;
					bootstrapError = err instanceof Error ? err.message : 'Failed to start a new chat';
				});
		};

		tryBootstrap();
		const timer = setInterval(tryBootstrap, 150);
		return () => {
			cancelled = true;
			clearInterval(timer);
		};
	});
</script>

<div class="chat-home hero-layout">
	<div class="empty-region">
		<EmptyChatState />
		{#if bootMessage || bootstrapError}
			<div class="boot-error-wrap">
				<p class="boot-error">{bootstrapError || bootMessage}</p>
				{#if bootMessage}
					<button class="set-workspace-button" onclick={openSettings}>
						<FolderOpen size={14} />
						Set workspace
					</button>
				{/if}
			</div>
		{/if}
	</div>

	<div class="composer-wrapper centered">
		<HeroComposerFrame>
			<Composer bind:this={composerRef} onSend={() => {}} disabled={true} variant="hero" />
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
		/* Cap the column to the container so the fixed-width hero composer shrinks
		   to fit instead of overflowing and being clipped by `.chat-home`'s
		   `overflow: hidden` when the window (or main pane, with the sidebar open)
		   is narrow. */
		grid-template-columns: minmax(0, 1fr);
		place-items: center;
		align-content: center;
		gap: clamp(1.5rem, 6cqi, 52px);
		padding: clamp(1rem, 5cqi, 48px);
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
	}

	.chat-home.hero-layout .composer-wrapper {
		position: relative;
		bottom: auto;
		left: auto;
		transform: none;
		width: 100%;
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
		padding: 0 var(--chat-gutter);
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		justify-content: center;
	}

	.chat-home.hero-layout .composer-wrapper :global(.hero-composer-frame) {
		width: min(var(--chat-composer-width), 100%);
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
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

	@media (max-width: 900px) {
		.chat-home.hero-layout {
			gap: 40px;
			padding: 32px 28px;
		}
	}
</style>
