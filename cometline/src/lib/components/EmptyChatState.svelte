<script lang="ts">
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { resolvePersona, personaAvatarSrcset as builtinAvatarSrcset } from '$lib/personas';
	import { personaAvatarCache } from '$lib/personas/avatar-cache.svelte';

	let resolvedPersona = $derived(
		resolvePersona(settingsStore.settings.app.personaId, settingsStore.settings.app.personas.custom)
	);
	let avatarSrc = $derived(personaAvatarCache.avatarSrcFor(resolvedPersona, 192));
	let avatarSrcset = $derived(
		resolvedPersona.kind === 'builtin' ? builtinAvatarSrcset(resolvedPersona) : undefined
	);
</script>

<div class="empty-state">
	<div class="avatar rounded-full border border-gray-400" aria-hidden="true">
		<img src={avatarSrc} srcset={avatarSrcset} sizes="82px" alt="" />
	</div>
	<p class="subtitle">A thought, a task, a file — Cometline continues.</p>
</div>

<style>
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		text-align: center;
	}

	.avatar {
		width: 82px;
		height: 82px;
		background: #fff;
		margin-bottom: 24px;
		box-shadow: var(--shadow-card);
		overflow: hidden;
	}

	.avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
		display: block;
	}

	.subtitle {
		font-size: 14px;
		color: var(--text-muted);
		margin: 0;
	}
</style>
