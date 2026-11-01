<script lang="ts">
	let { catalogId }: { catalogId: string } = $props();
</script>

<div class="connector-icon" data-id={catalogId} aria-hidden="true">
	{#if catalogId === 'atlassian'}
		<svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
			<rect width="32" height="32" rx="8" fill="#1868DB" />
			<path
				d="M16.2 19.8 11 14.6c-.4-.4-.4-1 0-1.4l1.5-1.5c.4-.4 1-.4 1.4 0l2.3 2.3 5.8-5.8c.4-.4 1-.4 1.4 0l1.5 1.5c.4.4.4 1 0 1.4l-7.5 7.5c-.4.4-1 .4-1.4 0Z"
				fill="#fff"
			/>
		</svg>
	{:else if catalogId === 'heptabase'}
		<svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
			<rect width="32" height="32" rx="8" fill="#F4F1EA" />
			<path
				d="M16 7.5 22.5 11v10L16 24.5 9.5 21V11L16 7.5Z"
				stroke="#2B2B2B"
				stroke-width="1.6"
				stroke-linejoin="round"
			/>
			<circle cx="16" cy="16" r="2.2" fill="#E85D04" />
		</svg>
	{:else if catalogId === 'linear'}
		<svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
			<rect width="32" height="32" rx="8" fill="#5E6AD2" />
			<path
				d="M11 21.5 21 11.5M21 21.5 11 11.5"
				stroke="#fff"
				stroke-width="2.4"
				stroke-linecap="round"
			/>
		</svg>
	{:else if catalogId === 'sentry'}
		<svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
			<rect width="32" height="32" rx="8" fill="#1D1127" />
			<path
				d="M9 19.5c2.2-4.8 5.8-7.5 10.5-8.2-.3 1.8-.1 3.6.6 5.2M23 19.5c-1.4-2.8-3.8-4.9-6.8-5.8 1.9-.2 3.7.2 5.3 1.2"
				stroke="#E1567C"
				stroke-width="1.8"
				stroke-linecap="round"
				stroke-linejoin="round"
			/>
		</svg>
	{:else}
		<svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
			<rect width="32" height="32" rx="8" fill="rgba(15,23,42,0.08)" />
			<circle cx="16" cy="16" r="5" fill="rgba(15,23,42,0.35)" />
		</svg>
	{/if}
</div>

<style>
	.connector-icon {
		width: 40px;
		height: 40px;
		flex-shrink: 0;
	}

	.connector-icon :global(svg) {
		display: block;
		width: 100%;
		height: 100%;
	}
</style>
