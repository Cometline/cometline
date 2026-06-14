<script lang="ts">
	import { flyUserBubble, type FlyUserBubbleParams } from '$lib/first-turn-flight';

	interface RunOptions {
		onPrepare?: () => void;
		skipOnPrepare?: boolean;
		textareaFrom?: DOMRect | null;
		deferReveal?: boolean;
		skipStage?: boolean;
	}

	interface Props {
		root: HTMLElement | null;
		stageUser: (text: string) => void;
		revealStagedUser: () => void;
	}

	let { root, stageUser, revealStagedUser }: Props = $props();

	let userFlightStyle = $state('');
	let userFlightText = $state('');
	let showUserFlight = $state(false);

	function showParticle(text: string, style: string) {
		userFlightText = text;
		userFlightStyle = style;
		showUserFlight = true;
	}

	function hideParticle() {
		showUserFlight = false;
		userFlightText = '';
		userFlightStyle = '';
	}

	function flightParams(text: string, opts: RunOptions = {}): FlyUserBubbleParams | null {
		if (!root) return null;
		return {
			root,
			text,
			stageUser,
			revealStagedUser,
			onPrepare: opts.onPrepare,
			skipOnPrepare: opts.skipOnPrepare,
			textareaFrom: opts.textareaFrom,
			deferReveal: opts.deferReveal,
			skipStage: opts.skipStage,
			onShowParticle: showParticle,
			onHideParticle: hideParticle
		};
	}

	export function run(text: string, opts: RunOptions = {}): void {
		void runAsync(text, opts);
	}

	export async function runAsync(text: string, opts: RunOptions = {}): Promise<boolean> {
		const params = flightParams(text, opts);
		if (!params) {
			stageUser(text);
			revealStagedUser();
			return false;
		}
		return flyUserBubble(params);
	}
</script>

{#if showUserFlight}
	<div class="flight-particle user-flight" style={userFlightStyle}>{userFlightText}</div>
{/if}

<style>
	.flight-particle {
		position: fixed;
		z-index: 40;
		pointer-events: none;
		transform-origin: top left;
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
