<script lang="ts">
	import { tick } from 'svelte';
	import UserBubbleFlight from '$lib/components/UserBubbleFlight.svelte';
	import { afterPaint, FLIGHT_MS, rectStyle, waitForSelector } from '$lib/first-turn-flight';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { resolvePersona, personaAvatarSrcset as builtinAvatarSrcset } from '$lib/personas';
	import { personaAvatarCache } from '$lib/personas/avatar-cache.svelte';
	import type { ImageAttachment } from '$lib/types';

	interface Props {
		root: HTMLElement | null;
		userBubbleFlight: UserBubbleFlight;
		stageUser: (text: string, images?: ImageAttachment[]) => void;
		revealStagedUser: () => void;
		onActiveChange?: (active: boolean) => void;
		onFlightDoneChange?: (done: boolean) => void;
		onPrepareFlight?: () => void;
		onComplete?: () => void;
	}

	interface RunOptions {
		visualOnly?: boolean;
		stageUser?: (text: string, images?: ImageAttachment[]) => void;
		revealStagedUser?: () => void;
	}

	let {
		root,
		userBubbleFlight,
		stageUser,
		revealStagedUser,
		onActiveChange,
		onFlightDoneChange,
		onPrepareFlight,
		onComplete
	}: Props = $props();

	let active = $state(false);
	let avatarFlightElement = $state<HTMLDivElement | null>(null);
	let avatarFlightStyle = $state('');
	let resolvedPersona = $derived(
		resolvePersona(settingsStore.settings.app.personaId, settingsStore.settings.app.personas.custom)
	);
	let avatarSrc = $derived(personaAvatarCache.avatarSrcFor(resolvedPersona, 192));
	let avatarSrcset = $derived(
		resolvedPersona.kind === 'builtin' ? builtinAvatarSrcset(resolvedPersona) : undefined
	);
	let showAvatarFlight = $state(false);

	export function run(text: string, images?: ImageAttachment[], opts: RunOptions = {}): void {
		if (active) return;
		void animate(text, images, opts);
	}

	export async function runAsync(
		text: string,
		images?: ImageAttachment[],
		opts: RunOptions = {}
	): Promise<void> {
		if (active) return;
		await animate(text, images, opts);
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

	async function waitForAvatarFlightEnd(): Promise<void> {
		await tick();
		const element = avatarFlightElement;
		if (!element) return;

		await new Promise<void>((resolve) => {
			let done = false;
			const finish = () => {
				if (done) return;
				done = true;
				element.removeEventListener('animationend', finish);
				window.clearTimeout(timeout);
				resolve();
			};
			const timeout = window.setTimeout(finish, FLIGHT_MS + 160);
			element.addEventListener('animationend', finish, { once: true });
		});
	}

	async function waitForStableRect(element: HTMLElement): Promise<DOMRect> {
		let rect = element.getBoundingClientRect();
		let stableFrames = 0;

		for (let frame = 0; frame < 40; frame++) {
			await afterPaint();
			const next = element.getBoundingClientRect();
			const stable =
				Math.abs(next.left - rect.left) < 0.5 &&
				Math.abs(next.top - rect.top) < 0.5 &&
				Math.abs(next.width - rect.width) < 0.5 &&
				Math.abs(next.height - rect.height) < 0.5;

			if (stable) {
				stableFrames += 1;
				if (stableFrames >= 2) return next;
			} else {
				stableFrames = 0;
			}

			rect = next;
		}

		return rect;
	}

	async function animate(
		text: string,
		images?: ImageAttachment[],
		opts: RunOptions = {}
	): Promise<void> {
		const visualOnly = opts.visualOnly ?? false;
		const runStageUser = opts.stageUser ?? stageUser;
		const runRevealStagedUser = opts.revealStagedUser ?? revealStagedUser;

		if (!root) {
			if (!visualOnly) {
				runStageUser(text, images);
				runRevealStagedUser();
			}
			setFlightDone(true);
			setActive(false);
			onComplete?.();
			return;
		}

		const emptyAvatar = root.querySelector('.empty-state .avatar');
		const textarea = root.querySelector('.composer .rce-editor');
		const avatarFrom =
			emptyAvatar instanceof HTMLElement ? emptyAvatar.getBoundingClientRect() : null;
		const textareaFrom =
			textarea instanceof HTMLElement ? textarea.getBoundingClientRect() : null;

		onPrepareFlight?.();
		setActive(true);
		setFlightDone(false);
		if (!visualOnly) runStageUser(text, images);
		await tick();

		let avatarFlightEnd: Promise<void> | undefined;
		const avatarTarget = await waitForSelector(root, '[data-flight-target="avatar"]');
		if (avatarFrom && avatarTarget instanceof HTMLElement) {
			const avatarTo = await waitForStableRect(avatarTarget);
			avatarFlightStyle = rectStyle(avatarFrom, avatarTo);
			showAvatarFlight = true;
			avatarFlightEnd = waitForAvatarFlightEnd();
		}

		const userFlew = await userBubbleFlight.runAsync(text, images, {
			skipOnPrepare: true,
			skipStage: true,
			skipReveal: visualOnly,
			textareaFrom,
			deferReveal: !visualOnly,
			deferHideParticle: true
		});

		if (!userFlew) {
			await avatarFlightEnd;
			if (!visualOnly) runRevealStagedUser();
			setFlightDone(true);
			await afterPaint();
			hideAvatarParticle();
			userBubbleFlight.dismissParticle();
			setActive(false);
			onComplete?.();
			return;
		}

		await avatarFlightEnd;
		if (!visualOnly) runRevealStagedUser();
		// Unhide the real thread avatar slot BEFORE tearing down flight particles,
		// so the avatar never blinks out between overlay end and the thread slot.
		setFlightDone(true);
		await afterPaint();

		hideAvatarParticle();
		userBubbleFlight.dismissParticle();
		setActive(false);
		onComplete?.();
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
