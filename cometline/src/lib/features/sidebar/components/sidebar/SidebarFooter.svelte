<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Settings, Briefcase, Sparkles, Bell, Images, CircleDollarSign } from '@lucide/svelte';
	import { inboxStore } from '#lib/stores/inbox.svelte.js';
	import { jobsIndicatorStore } from '#lib/stores/jobs-indicator.svelte.js';
	import { skillDraftsStore } from '#lib/stores/skill-drafts.svelte.js';
	import { openSettings } from '#lib/actions/open-settings.js';
	import Tooltip from '#lib/components/Tooltip.svelte';
</script>

<div class="sidebar-footer p-2">
	<Tooltip label="Settings" action="openSettings">
		<button aria-label="Settings" onclick={openSettings}>
			<Settings size={16} stroke-width={1.8} />
		</button>
	</Tooltip>
	<Tooltip label="Jobs" action="openJobs">
		<button
			aria-label={jobsIndicatorStore.hasOngoing
				? `Jobs (${jobsIndicatorStore.ongoingCount} ongoing)`
				: 'Jobs'}
			class="nav-badge"
			class:has-badge={jobsIndicatorStore.hasOngoing}
			class:active={page.url.pathname === '/jobs'}
			onclick={() => goto(resolve('jobs'))}><Briefcase size={16} stroke-width={1.8} /></button
		>
	</Tooltip>
	<Tooltip label="Skills" action="openSkillDrafts">
		<button
			aria-label="Skills"
			class="nav-badge"
			class:has-badge={skillDraftsStore.hasDrafts}
			class:active={page.url.pathname === '/skills' || page.url.pathname === '/skill-drafts'}
			onclick={() => goto(resolve('skills'))}
			><Sparkles size={16} stroke-width={1.8} /></button
		>
	</Tooltip>
	<Tooltip label="Gallery" action="openGallery">
		<button
			aria-label="Gallery"
			class="nav-badge"
			class:active={page.url.pathname === '/gallery'}
			onclick={() => goto(resolve('gallery'))}><Images size={16} stroke-width={1.8} /></button
		>
	</Tooltip>
	<Tooltip label="Usage" action="openUsage">
		<button
			aria-label="Usage"
			class="nav-badge"
			class:active={page.url.pathname === '/usage'}
			onclick={() => goto(resolve('usage'))}
			><CircleDollarSign size={16} stroke-width={1.8} /></button
		>
	</Tooltip>
	<Tooltip label="Inbox" action="openInbox">
		<button
			aria-label="Inbox"
			class="nav-badge"
			class:has-badge={inboxStore.openCount > 0}
			class:active={inboxStore.drawerOpen}
			onclick={() => inboxStore.toggleDrawer()}
		>
			<Bell size={16} stroke-width={1.8} />
		</button>
	</Tooltip>
</div>

<style>
	.sidebar-footer button {
		width: 28px;
		height: 28px;
		border: none;
		background: transparent;
		border-radius: 6px;
		color: var(--text-muted);
		display: grid;
		place-items: center;
	}

	.sidebar-footer button:hover {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.sidebar-footer button:active {
		background: rgba(0, 0, 0, 0.07);
	}

	.sidebar-footer button.active {
		background: rgba(0, 0, 0, 0.1);
	}

	.sidebar-footer {
		margin-top: auto;
		margin-right: 10px;
		margin-left: 10px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
		display: flex;
		flex-direction: row;
		gap: 4px;
	}

	.sidebar-footer .nav-badge {
		position: relative;
	}

	.sidebar-footer .nav-badge.has-badge::after {
		content: '';
		position: absolute;
		top: 4px;
		right: 4px;
		width: 5px;
		height: 5px;
		border-radius: 999px;
		background: var(--accent);
	}
</style>
