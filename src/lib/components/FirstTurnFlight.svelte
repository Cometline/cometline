<script lang="ts">
	import { tick } from 'svelte';
	import UserBubbleFlight from '$lib/components/UserBubbleFlight.svelte';
	import {
		FLIGHT_MS,
		afterPaint,
		rectStyle,
		wait,
		waitForSelector
	} from '$lib/first-turn-flight';

	interface Props {
		root: HTMLElement | null;
		userBubbleFlight: UserBubbleFlight;
		stageUser: (text: string) => void;
		revealStagedUser: () => void;
		onActiveChange?: (active: boolean) => void;
		onFlightDoneChange?: (done: boolean) => void;
		onPrepareFlight?: () => void;
		onComplete?: () => void;
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
	let avatarFlightStyle = $state('');
	let showAvatarFlight = $state(false);

	export function run(text: string): void {
		if (active) return;
		void animate(text);
	}

	function setActive(value: boolean) {
		active = value;
		onActiveChange?.(value);
	}

	function setFlightDone(value: boolean) {
		onFlightDoneChange?.(value);
	}

	async function animateAvatar(avatarFrom: DOMRect, avatarTarget: HTMLElement): Promise<void> {
		const avatarTo = avatarTarget.getBoundingClientRect();
		avatarFlightStyle = rectStyle(avatarFrom, avatarTo);
		showAvatarFlight = true;
		await wait(FLIGHT_MS);
		showAvatarFlight = false;
		avatarFlightStyle = '';
	}

	async function animate(text: string): Promise<void> {
		if (!root) {
			stageUser(text);
			revealStagedUser();
			setFlightDone(true);
			setActive(false);
			onComplete?.();
			return;
		}

		const emptyAvatar = root.querySelector('.empty-state .avatar');
		const textarea = root.querySelector('.composer textarea');
		const avatarFrom =
			emptyAvatar instanceof HTMLElement ? emptyAvatar.getBoundingClientRect() : null;
		const textareaFrom =
			textarea instanceof HTMLElement ? textarea.getBoundingClientRect() : null;

		onPrepareFlight?.();
		setActive(true);
		setFlightDone(false);
		stageUser(text);
		await tick();

		const avatarTarget = await waitForSelector(root, '[data-flight-target="avatar"]');

		const animations: Promise<unknown>[] = [
			userBubbleFlight.runAsync(text, {
				skipOnPrepare: true,
				skipStage: true,
				textareaFrom,
				deferReveal: true
			})
		];

		if (avatarFrom && avatarTarget instanceof HTMLElement) {
			animations.push(animateAvatar(avatarFrom, avatarTarget));
		}

		const results = await Promise.all(animations);
		const userFlew = results[0] === true;

		if (!userFlew) {
			revealStagedUser();
			setFlightDone(true);
			setActive(false);
			onComplete?.();
			return;
		}

		revealStagedUser();
		setFlightDone(true);
		await afterPaint();
		setActive(false);
		onComplete?.();
	}
</script>

{#if showAvatarFlight}
	<div
		class="flight-particle avatar-flight rounded-full border border-gray-400 overflow-hidden"
		style={avatarFlightStyle}
	>
		<img
			src="/project_avatar_192.png"
			srcset="/project_avatar_96.png 96w, /project_avatar_192.png 192w, /project_avatar_384.png 384w"
			sizes="82px"
			alt=""
		/>
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
