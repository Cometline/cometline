<script lang="ts">
	import {
		flyUserBubble,
		type FlyUserBubbleParams,
		type UserBubbleFlightOrigin
	} from '$lib/first-turn-flight';
	import { imageDataURL } from '$lib/files/images';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import type { ImageAttachment } from '$lib/types';

	interface RunOptions {
		onPrepare?: () => void;
		skipOnPrepare?: boolean;
		textareaFrom?: DOMRect | null;
		deferReveal?: boolean;
		deferHideParticle?: boolean;
		skipStage?: boolean;
		origin?: UserBubbleFlightOrigin;
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
			stageUser,
			revealStagedUser,
			onPrepare: opts.onPrepare,
			skipOnPrepare: opts.skipOnPrepare,
			textareaFrom: opts.textareaFrom,
			deferReveal: opts.deferReveal,
			deferHideParticle: opts.deferHideParticle,
			skipStage: opts.skipStage,
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
			stageUser(text, images);
			revealStagedUser();
			return false;
		}
		return flyUserBubble(params);
	}
</script>

{#if showUserFlight}
	<div class="flight-particle user-flight" style={userFlightStyle}>
		{#if userFlightImages?.length}
			<div class="flight-images" class:text-following={Boolean(userFlightText)}>
				{#each userFlightImages as image, index (`${image.id ?? index}`)}
					<img src={imageDataURL(image)} alt={image.name ?? 'Attached image'} />
				{/each}
			</div>
		{/if}
		{#if userFlightText.trim()}
			<AssistantMarkdown source={userFlightText.trim()} mode="user" />
		{/if}
	</div>
{/if}

<style>
	.flight-particle {
		position: fixed;
		z-index: 40;
		pointer-events: none;
		transform-origin: top left;
	}

	.user-flight {
		width: fit-content;
		max-width: var(--chat-content-column, min(360px, 72vw));
		padding: 11px 14px;
		border-radius: 18px 18px 6px 18px;
		background: var(--user-bubble-bg);
		color: white;
		font-size: 14px;
		line-height: 1.55;
		/* The inner AssistantMarkdown renders chips + preserves text newlines via
		 * `.markdown.user-text { white-space: pre-wrap }`. Keep the wrapper at
		 * `normal` so template whitespace doesn't inflate the particle (matches
		 * the final `.user-bubble`), so there's no visible swap on handoff. */
		white-space: normal;
		word-break: normal;
		box-shadow: 0 16px 40px var(--user-bubble-shadow);
		animation: user-bubble-flight var(--duration-flight) var(--ease-smooth) forwards;
	}

	.flight-images {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(96px, 1fr));
		gap: 8px;
		max-width: min(360px, 72vw);
	}

	.flight-images.text-following {
		margin-bottom: 8px;
	}

	.flight-images img {
		width: 100%;
		max-height: 220px;
		object-fit: cover;
		border-radius: 12px;
		border: 1px solid rgba(255, 255, 255, 0.35);
		display: block;
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
