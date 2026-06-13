<script lang="ts">
	import { tick } from 'svelte';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/Composer.svelte';
	import ChatThread from '$lib/components/ChatThread.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { getSession } from '$lib/client/cometmind';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import {
		FLIGHT_MS,
		afterPaint,
		rectStyle,
		textareaUserOrigin,
		wait,
		waitForSelector
	} from '$lib/first-turn-flight';

	let {
		sessionId,
		bootMessage = '',
		initialMessage = null
	}: {
		sessionId: string;
		bootMessage?: string;
		initialMessage?: string | null;
	} = $props();

	let chatHome: HTMLDivElement;
	let awaitingFirstAssistant = $state(false);
	let firstTurnActive = $state(false);
	let firstTurnFlightDone = $state(false);
	let userFlightStyle = $state('');
	let userFlightText = $state('');
	let avatarFlightStyle = $state('');
	let showUserFlight = $state(false);
	let showAvatarFlight = $state(false);
	let bootstrapped = $state(false);

	let hasVisibleConversation = $derived(chatStore.items.length > 0 || chatStore.isLoading);
	let composerVariant = $derived<'hero' | 'dock'>(shellStore.composerPhase === 'centered' ? 'hero' : 'dock');
	let heroLayout = $derived(
		!hasVisibleConversation && shellStore.composerPhase === 'centered' && !firstTurnActive
	);

	$effect(() => {
		if (sessionStore.current?.id !== sessionId) return;
		modelStore.selectFromSession(sessionStore.current);
	});

	$effect(() => {
		const id = sessionId;
		if (initialMessage) return;
		if (chatStore.isStreaming && chatStore.sessionID === id) return;
		void chatStore.loadTranscript(id);
	});

	$effect(() => {
		if (!hasVisibleConversation && !firstTurnActive) {
			awaitingFirstAssistant = false;
			firstTurnFlightDone = false;
		}
	});

	$effect(() => {
		if (hasVisibleConversation && !firstTurnActive && !initialMessage) {
			shellStore.dockComposer();
		}
	});

	$effect(() => {
		if (bootstrapped || !initialMessage) return;
		bootstrapped = true;
		void sendFirstTurn(initialMessage);
	});

	async function runParallelFirstTurn(text: string): Promise<void> {
		const emptyAvatar = chatHome?.querySelector('.empty-state .avatar');
		const textarea = chatHome?.querySelector('.composer textarea');
		const avatarFrom =
			emptyAvatar instanceof HTMLElement ? emptyAvatar.getBoundingClientRect() : null;
		const textareaFrom =
			textarea instanceof HTMLElement ? textarea.getBoundingClientRect() : null;

		firstTurnActive = true;
		firstTurnFlightDone = false;
		awaitingFirstAssistant = true;

		shellStore.dockComposer();
		chatStore.stageUser(text);
		await tick();

		const userTarget = await waitForSelector(chatHome, '[data-flight-target="user"]');
		const avatarTarget = await waitForSelector(chatHome, '[data-flight-target="avatar"]');

		if (
			userTarget instanceof HTMLElement &&
			avatarTarget instanceof HTMLElement &&
			textareaFrom &&
			avatarFrom
		) {
			const userTo = userTarget.getBoundingClientRect();
			const avatarTo = avatarTarget.getBoundingClientRect();
			userFlightText = text;
			userFlightStyle = rectStyle(textareaUserOrigin(textareaFrom, userTo), userTo);
			avatarFlightStyle = rectStyle(avatarFrom, avatarTo);
			showUserFlight = true;
			showAvatarFlight = true;
			await wait(FLIGHT_MS);

			chatStore.revealStagedUser();
			firstTurnFlightDone = true;
			await afterPaint();

			showUserFlight = false;
			showAvatarFlight = false;
			userFlightText = '';
		} else {
			chatStore.revealStagedUser();
			firstTurnFlightDone = true;
		}

		firstTurnActive = false;
	}

	async function sendFirstTurn(text: string) {
		const firstTurn = !hasVisibleConversation;
		const flightTask = firstTurn ? runParallelFirstTurn(text) : Promise.resolve();
		await flightTask;
		await chatStore.send(sessionId, text, { skipUser: firstTurn });
		if (firstTurn) awaitingFirstAssistant = false;
		await refreshSession();
	}

	async function onSend(text: string) {
		const firstTurn = !hasVisibleConversation;
		const flightTask = firstTurn ? runParallelFirstTurn(text) : Promise.resolve();
		await flightTask;
		await chatStore.send(sessionId, text, { skipUser: firstTurn });
		if (firstTurn) awaitingFirstAssistant = false;
		await refreshSession();
	}

	async function refreshSession() {
		try {
			sessionStore.updateSession(await getSession(sessionId));
		} catch {
			// The transcript is the source of truth; title refresh is best effort.
		}
	}
</script>

<div
	class="chat-home"
	class:hero-layout={heroLayout}
	class:first-turn-active={firstTurnActive}
	bind:this={chatHome}
>
	{#if !hasVisibleConversation && !firstTurnActive}
		<div class="empty-region">
			<EmptyChatState />
			{#if bootMessage}
				<p class="boot-error">{bootMessage}</p>
			{/if}
		</div>
	{:else}
		<ChatThread {awaitingFirstAssistant} {firstTurnFlightDone} />
	{/if}

	{#if showUserFlight}
		<div class="flight-particle user-flight" style={userFlightStyle}>{userFlightText}</div>
	{/if}
	{#if showAvatarFlight}
		<div class="flight-particle avatar-flight" style={avatarFlightStyle}>
			<img src="/project_icon.png" alt="" />
		</div>
	{/if}

	<div class="composer-wrapper" class:centered={shellStore.composerPhase === 'centered'}>
		<Composer
			onSend={onSend}
			disabled={chatStore.isStreaming || connectionState.status !== 'ready'}
			variant={composerVariant}
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

	.chat-home.hero-layout .empty-region {
		position: static;
		inset: unset;
		flex: 0 0 auto;
		padding: 0;
	}

	.empty-region {
		position: absolute;
		inset: 0 0 180px;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 48px 48px 0;
		flex-direction: column;
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
		transition:
			bottom 560ms cubic-bezier(0.22, 1, 0.36, 1),
			transform 560ms cubic-bezier(0.22, 1, 0.36, 1),
			width 560ms cubic-bezier(0.22, 1, 0.36, 1);
	}

	.composer-wrapper.centered {
		bottom: 50%;
		transform: translateX(-50%) translateY(168px);
		width: min(700px, calc(100% - 64px));
	}

	.flight-particle {
		position: fixed;
		z-index: 40;
		pointer-events: none;
		transform-origin: top left;
		animation: first-turn-flight 560ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
	}

	.user-flight {
		padding: 11px 14px;
		border-radius: 18px 18px 6px 18px;
		background: #1f2933;
		color: white;
		font-size: 14px;
		line-height: 1.55;
		white-space: pre-wrap;
		word-break: break-word;
		box-shadow: 0 16px 40px rgba(31, 41, 51, 0.18);
	}

	.avatar-flight {
		border-radius: 24px;
		background: linear-gradient(145deg, #ffffff, #eef2f6);
		border: 1px solid var(--border-soft);
		box-shadow: 0 8px 28px rgba(15, 23, 42, 0.1);
		padding: 4px;
		overflow: hidden;
	}

	.avatar-flight img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 20px;
		display: block;
	}

	@keyframes first-turn-flight {
		from {
			transform: translate3d(0, 0, 0) scale(1, 1);
		}
		to {
			transform: translate3d(var(--flight-x), var(--flight-y), 0) scale(var(--flight-sx), var(--flight-sy));
		}
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

		.empty-region {
			inset: 0 0 160px;
			padding-inline: 28px;
		}
	}
</style>
