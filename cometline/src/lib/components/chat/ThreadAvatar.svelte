<script lang="ts">
	let {
		variant,
		avatarSrc,
		avatarSrcset,
		flightHidden = false,
		flightTarget
	}: {
		variant: 'avatar' | 'gutter';
		avatarSrc: string;
		avatarSrcset?: string;
		flightHidden?: boolean;
		flightTarget?: string;
	} = $props();
</script>

{#if variant === 'avatar'}
	<div
		class="avatar-mini"
		class:avatar-flight-hidden={flightHidden}
		data-flight-target={flightTarget}
		style:visibility={flightHidden ? 'hidden' : undefined}
	>
		<img
			src={avatarSrc}
			srcset={avatarSrcset}
			sizes="(min-width: 1280px) 48px, (min-width: 1024px) 44px, (min-width: 768px) 40px, 36px"
			alt=""
		/>
	</div>
{:else}
	<div class="avatar-gutter" aria-hidden="true"></div>
{/if}

<style>
	.avatar-mini,
	.avatar-gutter {
		flex: 0 0 auto;
		width: var(--chat-avatar-size);
		height: var(--chat-avatar-size);
	}

	.avatar-mini {
		aspect-ratio: 1;
		border-radius: 50%;
		border: 1px solid #9ca3af;
		background: linear-gradient(145deg, #ffffff, #eef2f6);
		box-shadow: 0 5px 14px rgba(15, 23, 42, 0.06);
		overflow: hidden;
	}

	@media (min-width: 1024px) {
		.avatar-mini {
			box-shadow: 0 6px 18px rgba(15, 23, 42, 0.08);
		}
	}

	.avatar-mini img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
		display: block;
	}

	.avatar-flight-hidden {
		visibility: hidden;
	}
</style>
