<script lang="ts">
	import { tick } from 'svelte';
	import {
		FLIGHT_MS,
		afterPaint,
		rectStyle,
		translateStyle,
		textareaUserOrigin,
		wait,
		waitForSelector
	} from '$lib/first-turn-flight';

	interface Props {
		root: HTMLElement | null;
		stageUser: (text: string) => void;
		revealStagedUser: () => void;
		onActiveChange?: (active: boolean) => void;
		onFlightDoneChange?: (done: boolean) => void;
		onComplete?: () => void;
	}

	let {
		root,
		stageUser,
		revealStagedUser,
		onActiveChange,
		onFlightDoneChange,
		onComplete
	}: Props = $props();

	let active = $state(false);
	let flightDone = $state(false);
	let userFlightStyle = $state('');
	let userFlightText = $state('');
	let avatarFlightStyle = $state('');
	let showUserFlight = $state(false);
	let showAvatarFlight = $state(false);

	export function run(text: string): void {
		if (active) return;
		setActive(true);
		setFlightDone(false);
		void animate(text);
	}

	function setActive(value: boolean) {
		active = value;
		onActiveChange?.(value);
	}

	function setFlightDone(value: boolean) {
		flightDone = value;
		onFlightDoneChange?.(value);
	}

	async function animate(text: string): Promise<void> {
		if (!root) {
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

		stageUser(text);
		await tick();

		const userTarget = await waitForSelector(root, '[data-flight-target="user"]');
		const avatarTarget = await waitForSelector(root, '[data-flight-target="avatar"]');

		if (
			userTarget instanceof HTMLElement &&
			avatarTarget instanceof HTMLElement &&
			textareaFrom &&
			avatarFrom
		) {
			const userTo = userTarget.getBoundingClientRect();
			const avatarTo = avatarTarget.getBoundingClientRect();
			userFlightText = text;
			userFlightStyle = translateStyle(textareaUserOrigin(textareaFrom, userTo), userTo);
			avatarFlightStyle = rectStyle(avatarFrom, avatarTo);
			showUserFlight = true;
			showAvatarFlight = true;
			await wait(FLIGHT_MS);

			revealStagedUser();
			// Unhide the real thread avatar slot (via onFlightDoneChange) BEFORE
			// tearing down the flight overlay below, so the avatar never blinks
			// out between the overlay ending and the thread placeholder showing.
			setFlightDone(true);
			await afterPaint();

			showUserFlight = false;
			showAvatarFlight = false;
			userFlightText = '';
		} else {
			revealStagedUser();
			setFlightDone(true);
		}

		setActive(false);
		onComplete?.();
	}
</script>

{#if showUserFlight}
	<div class="flight-particle user-flight" style={userFlightStyle}>{userFlightText}</div>
{/if}
{#if showAvatarFlight}
	<div class="flight-particle avatar-flight" style={avatarFlightStyle}>
		<img src="/project_icon.png" alt="" />
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
		animation-name: first-turn-user-flight;
	}

	.avatar-flight {
		border-radius: 50%;
		background: linear-gradient(145deg, #ffffff, #eef2f6);
		border: 1px solid var(--border-soft);
		box-shadow: 0 5px 14px rgba(15, 23, 42, 0.06);
		overflow: hidden;
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
			transform: translate3d(var(--flight-x), var(--flight-y), 0) scale(var(--flight-sx), var(--flight-sy));
		}
	}

	@keyframes first-turn-user-flight {
		from {
			transform: translate3d(0, 0, 0);
		}
		to {
			transform: translate3d(var(--flight-x), var(--flight-y), 0);
		}
	}
</style>
