<script lang="ts">
	import { tick } from 'svelte';
	import UserBubbleFlight from '$lib/components/UserBubbleFlight.svelte';
	import {
		afterPaint,
		animateElementToRect,
		measureStableRect,
		rectStyle,
		waitForAnimationEnd,
		waitForSelector
	} from '$lib/first-turn-flight';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { resolvePersona, personaAvatarSrcset as builtinAvatarSrcset } from '$lib/personas';
	import { personaAvatarCache } from '$lib/personas/avatar-cache.svelte';
	import type { ImageAttachment } from '$lib/types';

	interface Props {
		root: HTMLElement | null;
		userBubbleFlight: UserBubbleFlight;
		stageUser: (text: string, images?: ImageAttachment[]) => string;
		revealStagedUser: () => void;
		onActiveChange?: (active: boolean) => void;
		onFlightDoneChange?: (done: boolean) => void;
		onPrepareFlight?: () => void;
	}

	interface RunOptions {
		stageUser?: (text: string, images?: ImageAttachment[]) => string;
		revealStagedUser?: () => void;
		signal?: AbortSignal;
	}

	let {
		root,
		userBubbleFlight,
		stageUser,
		revealStagedUser,
		onActiveChange,
		onFlightDoneChange,
		onPrepareFlight
	}: Props = $props();

	let active = $state(false);
	let avatarFlightElement = $state<HTMLDivElement | null>(null);
	let avatarFlightStyle = $state('');
	let resolvedPersona = $derived(
		resolvePersona(
			settingsStore.settings.app.personaId,
			settingsStore.settings.app.personas.custom
		)
	);
	let avatarSrc = $derived(personaAvatarCache.avatarSrcFor(resolvedPersona, 192));
	let avatarSrcset = $derived(
		resolvedPersona.kind === 'builtin' ? builtinAvatarSrcset(resolvedPersona) : undefined
	);
	let showAvatarFlight = $state(false);

	export async function runAsync(
		text: string,
		images?: ImageAttachment[],
		opts: RunOptions = {}
	): Promise<void> {
		if (active) return;
		await animate(text, images, opts);
	}

	export function cancel() {
		hideAvatarParticle();
		userBubbleFlight.dismissParticle();
		setActive(false);
	}

	function setActive(value: boolean) {
		active = value;
		onActiveChange?.(value);
	}

	function setFlightDone(value: boolean) {
		onFlightDoneChange?.(value);
	}

	function hideAvatarParticle() {
		showAvatarFlight = false;
		avatarFlightElement = null;
		avatarFlightStyle = '';
	}

	async function animate(
		text: string,
		images?: ImageAttachment[],
		opts: RunOptions = {}
	): Promise<void> {
		const runStageUser = opts.stageUser ?? stageUser;
		const runRevealStagedUser = opts.revealStagedUser ?? revealStagedUser;
		const { signal } = opts;
		if (!root) {
			runStageUser(text, images);
			runRevealStagedUser();
			setFlightDone(true);
			setActive(false);
			return;
		}

		const emptyAvatar = root.querySelector('.empty-state .avatar');
		const textarea = root.querySelector('.composer .rce-editor');
		const avatarFrom =
			emptyAvatar instanceof HTMLElement ? emptyAvatar.getBoundingClientRect() : null;
		const textareaFrom =
			textarea instanceof HTMLElement ? textarea.getBoundingClientRect() : null;

		const composerElement = root.querySelector('.composer-wrapper');
		const composerFrom =
			composerElement instanceof HTMLElement ? composerElement.getBoundingClientRect() : null;
		onPrepareFlight?.();
		setActive(true);
		setFlightDone(false);
		const userItemId = runStageUser(text, images);
		await tick();
		const composerFlight =
			composerElement instanceof HTMLElement
				? animateElementToRect(composerElement, composerFrom)
				: Promise.resolve();

		let avatarFlightEnd: Promise<void> | undefined;
		const avatarTarget = await waitForSelector(
			root,
			'[data-flight-target="avatar"]',
			4000,
			undefined,
			signal
		);
		if (signal?.aborted) {
			runRevealStagedUser();
			return;
		}
		if (avatarFrom && avatarTarget instanceof HTMLElement) {
			const avatarTo = await measureStableRect(avatarTarget);
			avatarFlightStyle = rectStyle(avatarFrom, avatarTo);
			showAvatarFlight = true;
			avatarFlightEnd = waitForAnimationEnd(avatarFlightElement);
		}

		const userFlew = await userBubbleFlight.runAsync(text, images, {
			skipOnPrepare: true,
			skipStage: true,
			textareaFrom,
			deferReveal: true,
			deferHideParticle: true,
			targetUserId: userItemId,
			signal
		});
		if (signal?.aborted) {
			runRevealStagedUser();
			return;
		}

		if (!userFlew) {
			await avatarFlightEnd;
			await composerFlight;
			runRevealStagedUser();
			setFlightDone(true);
			await afterPaint();
			hideAvatarParticle();
			userBubbleFlight.dismissParticle();
			setActive(false);
			return;
		}

		await avatarFlightEnd;
		await composerFlight;
		runRevealStagedUser();
		// Unhide the real thread avatar slot BEFORE tearing down flight particles,
		// so the avatar never blinks out between overlay end and the thread slot.
		setFlightDone(true);
		await afterPaint();

		hideAvatarParticle();
		userBubbleFlight.dismissParticle();
		setActive(false);
	}
</script>

{#if showAvatarFlight}
	<div
		bind:this={avatarFlightElement}
		class="flight-particle avatar-flight rounded-full border border-gray-400 overflow-hidden"
		style={avatarFlightStyle}
	>
		<img src={avatarSrc} srcset={avatarSrcset} sizes="82px" alt="" />
	</div>
{/if}

<style>
	.flight-particle {
		position: fixed;
		z-index: 40;
		pointer-events: none;
		transform-origin: top left;
		animation: first-turn-flight var(--duration-flight) var(--ease-smooth) forwards;
	}

	.avatar-flight {
		background: linear-gradient(145deg, #ffffff, #eef2f6);
		box-shadow: 0 5px 14px rgba(15, 23, 42, 0.06);
	}

	.avatar-flight img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
		display: block;
	}

	@keyframes first-turn-flight {
		from {
			transform: translate3d(0, 0, 0) scale(1, 1);
		}
		to {
			transform: translate3d(var(--flight-x), var(--flight-y), 0)
				scale(var(--flight-sx), var(--flight-sy));
		}
	}
</style>
