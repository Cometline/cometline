import { builtinPersonaAvatarSrc } from './builtins';
import type { ResolvedPersona } from './index';

/**
 * Reactive cache for custom persona avatar images. Builtin persona avatars
 * are static bundled assets and don't need caching; custom persona avatars
 * live under `~/.cometmind/personas/<id>/` and must be read via IPC as a
 * base64 data URL (see `readWorkspaceFileForPreview` in electron/main.cjs for
 * the analogous, already-shipped pattern this mirrors).
 */
function createPersonaAvatarCache() {
	let cache = $state<Record<string, string>>({});
	const pending = new Set<string>();

	async function ensureCustomAvatar(id: string) {
		if (cache[id] || pending.has(id)) return;
		if (!window.electronAPI?.readPersonaAvatar) return;
		pending.add(id);
		try {
			const result = await window.electronAPI.readPersonaAvatar(id);
			if (result.ok) {
				cache = { ...cache, [id]: result.dataUrl };
			}
		} catch {
			// Swallow — avatar display falls back to the placeholder below.
		} finally {
			pending.delete(id);
		}
	}

	/**
	 * Returns the best-known avatar src for a resolved persona. For custom
	 * personas whose avatar hasn't been fetched yet, kicks off an async fetch
	 * (reactive — callers re-render once the cache updates) and returns a
	 * placeholder in the meantime.
	 */
	function avatarSrcFor(resolved: ResolvedPersona, size: 96 | 192 | 384 = 192): string {
		if (resolved.kind === 'builtin') {
			return builtinPersonaAvatarSrc(resolved.id, size);
		}
		const cached = cache[resolved.id];
		if (cached) return cached;
		void ensureCustomAvatar(resolved.id);
		return builtinPersonaAvatarSrc('minako', size);
	}

	function invalidate(id: string) {
		if (!(id in cache)) return;
		const next = { ...cache };
		delete next[id];
		cache = next;
	}

	return { avatarSrcFor, ensureCustomAvatar, invalidate };
}

export const personaAvatarCache = createPersonaAvatarCache();
