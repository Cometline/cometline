<script lang="ts">
	import {
		flyUserBubble,
		type FlyUserBubbleParams,
		type UserBubbleFlightOrigin
	} from '$lib/first-turn-flight';
	import { imageDataURL } from '$lib/files/images';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import UserMessageViewport from '$lib/components/chat/UserMessageViewport.svelte';
	import type { ImageAttachment } from '$lib/types';

	interface RunOptions {
		onPrepare?: () => void;
		skipOnPrepare?: boolean;
		textareaFrom?: DOMRect | null;
		deferReveal?: boolean;
		deferHideParticle?: boolean;
		skipStage?: boolean;
		skipReveal?: boolean;
		origin?: UserBubbleFlightOrigin;
		targetUserId?: string;
		signal?: AbortSignal;
	}

	interface Props {
		root: HTMLElement | null;
		stageUser: (text: string, images?: ImageAttachment[]) => void;
		revealStagedUser: () => void;
	}

	let { root, stageUser, revealStagedUser }: Props = $props();

	let userFlightStyle = $state('');
	let userFlightText = $state('');
	let userFlightImages = $state<ImageAttachment[] | undefined>();
	let showUserFlight = $state(false);

	function showParticle(text: string, images: ImageAttachment[] | undefined, style: string) {
		userFlightText = text;
		userFlightImages = images;
		userFlightStyle = style;
		showUserFlight = true;
	}

	function hideParticle() {
		showUserFlight = false;
		userFlightText = '';
		userFlightImages = undefined;
		userFlightStyle = '';
	}

	export function dismissParticle() {
		hideParticle();
	}

	function flightParams(
		text: string,
		images?: ImageAttachment[],
		opts: RunOptions = {}
	): FlyUserBubbleParams | null {
		if (!root) return null;
		return {
			root,
			text,
			images,
			targetUserId: opts.targetUserId,
			signal: opts.signal,
			stageUser,
			revealStagedUser,
			onPrepare: opts.onPrepare,
			skipOnPrepare: opts.skipOnPrepare,
			textareaFrom: opts.textareaFrom,
			deferReveal: opts.deferReveal,
			deferHideParticle: opts.deferHideParticle,
			skipStage: opts.skipStage,
			skipReveal: opts.skipReveal,
			origin: opts.origin,
			onShowParticle: showParticle,
			onHideParticle: hideParticle
		};
	}

	export function run(text: string, images?: ImageAttachment[], opts: RunOptions = {}): void {
		void runAsync(text, images, opts);
	}

	export async function runAsync(
		text: string,
		images?: ImageAttachment[],
		opts: RunOptions = {}
	): Promise<boolean> {
		const params = flightParams(text, images, opts);
		if (!params) {
			if (!opts.skipStage) stageUser(text, images);
			if (!opts.skipReveal) revealStagedUser();
			return false;
		}
		return flyUserBubble(params);
	}
</script>

{#if showUserFlight}
	<div class="flight-particle bubble user-bubble" style={userFlightStyle}>
		<UserMessageViewport interactive={false}>
			{#if userFlightImages?.length}
				<div class="user-images" class:text-following={Boolean(userFlightText)}>
					{#each userFlightImages as image, index (`${image.id ?? index}`)}
						<img src={imageDataURL(image)} alt={image.name ?? 'Attached image'} />
					{/each}
				</div>
			{/if}
			{#if userFlightText.trim()}
				<AssistantMarkdown source={userFlightText.trim()} mode="user" />
			{/if}
		</UserMessageViewport>
	</div>
{/if}

<style>
	.flight-particle {
		position: fixed;
		z-index: 40;
		pointer-events: none;
		transform-origin: top left;
		overflow: hidden;
		box-shadow: 0 16px 40px var(--user-bubble-shadow);
		animation: user-bubble-flight var(--duration-flight) var(--ease-smooth) forwards;
	}

	@keyframes user-bubble-flight {
		from {
			transform: translate3d(0, 0, 0);
		}
		to {
			transform: translate3d(var(--flight-x), var(--flight-y), 0);
		}
	}
</style>
