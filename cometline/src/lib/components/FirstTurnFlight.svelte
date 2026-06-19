<script lang="ts">
	import { tick } from 'svelte';
	import UserBubbleFlight from '$lib/components/UserBubbleFlight.svelte';
	import { afterPaint, rectStyle, waitForSelector } from '$lib/first-turn-flight';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { projectAvatarSrc, projectAvatarSrcset } from '$lib/project-icon';
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
	let iconVariant = $derived(settingsStore.settings.app.iconVariant);
	let showAvatarFlight = $state(false);

	export function run(text: string, images?: ImageAttachment[]): void {
		if (active) return;
		void animate(text, images);
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
		avatarFlightStyle = '';
	}

	async function animate(text: string, images?: ImageAttachment[]): Promise<void> {
		if (!root) {
			stageUser(text, images);
			revealStagedUser();
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
		stageUser(text, images);
		await tick();

		const avatarTarget = await waitForSelector(root, '[data-flight-target="avatar"]');
		if (avatarFrom && avatarTarget instanceof HTMLElement) {
			avatarFlightStyle = rectStyle(avatarFrom, avatarTarget.getBoundingClientRect());
			showAvatarFlight = true;
		}

		const userFlew = await userBubbleFlight.runAsync(text, images, {
			skipOnPrepare: true,
			skipStage: true,
			textareaFrom,
			deferReveal: true,
			deferHideParticle: true
		});

		if (!userFlew) {
			revealStagedUser();
			setFlightDone(true);
			hideAvatarParticle();
			userBubbleFlight.dismissParticle();
			setActive(false);
			onComplete?.();
			return;
		}

		revealStagedUser();
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
		class="flight-particle avatar-flight rounded-full border border-gray-400 overflow-hidden"
		style={avatarFlightStyle}
	>
		<img
			src={projectAvatarSrc(iconVariant, 192)}
			srcset={projectAvatarSrcset(iconVariant)}
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
