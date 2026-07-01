import type { CustomPersona } from '$lib/types';
import {
	BUILTIN_PERSONAS,
	isBuiltinPersonaId,
	builtinPersonaAvatarSrc,
	builtinPersonaAvatarSrcset,
	builtinPersonaSoulFilename,
	migratePersonaIdFromIconVariant,
	type BuiltinPersonaId,
	type BuiltinPersona
} from './builtins';

export {
	BUILTIN_PERSONAS,
	isBuiltinPersonaId,
	builtinPersonaAvatarSrc,
	builtinPersonaAvatarSrcset,
	builtinPersonaSoulFilename,
	migratePersonaIdFromIconVariant
};
export type { BuiltinPersonaId, BuiltinPersona, CustomPersona };

export type ResolvedPersona =
	| { kind: 'builtin'; id: BuiltinPersonaId; label: string }
	| { kind: 'custom'; id: string; label: string; persona: CustomPersona };

/** Resolves a `personaId` against the builtin registry and a list of custom personas. */
export function resolvePersona(
	personaId: string | undefined,
	customPersonas: CustomPersona[] | undefined
): ResolvedPersona {
	if (isBuiltinPersonaId(personaId)) {
		const builtin = BUILTIN_PERSONAS.find((persona) => persona.id === personaId)!;
		return { kind: 'builtin', id: builtin.id, label: builtin.label };
	}
	const custom = (customPersonas ?? []).find((persona) => persona.id === personaId);
	if (custom) {
		return { kind: 'custom', id: custom.id, label: custom.name, persona: custom };
	}
	// Unknown/missing personaId falls back to the default builtin.
	return { kind: 'builtin', id: 'minako', label: 'Minako' };
}

/**
 * Synchronous avatar src for a resolved persona. Builtin personas resolve to a
 * static bundled asset path. Custom personas have no static path (their avatar
 * lives under `~/.cometmind/personas/<id>/` and must be fetched asynchronously
 * via `electronAPI.readPersonaAvatar`); callers should use the persona avatar
 * store/helper for custom personas and fall back to this for a placeholder.
 */
export function personaAvatarSrc(resolved: ResolvedPersona, size: 96 | 192 | 384 = 192): string {
	if (resolved.kind === 'builtin') {
		return builtinPersonaAvatarSrc(resolved.id, size);
	}
	return builtinPersonaAvatarSrc('minako', size);
}

export function personaAvatarSrcset(resolved: ResolvedPersona): string {
	if (resolved.kind === 'builtin') {
		return builtinPersonaAvatarSrcset(resolved.id);
	}
	return builtinPersonaAvatarSrcset('minako');
}

export function normalizeCustomPersona(value: unknown): CustomPersona | null {
	if (!value || typeof value !== 'object') return null;
	const raw = value as Record<string, unknown>;
	const id = typeof raw.id === 'string' ? raw.id.trim() : '';
	const name = typeof raw.name === 'string' ? raw.name.trim() : '';
	const avatarPath = typeof raw.avatarPath === 'string' ? raw.avatarPath : '';
	const soulPath = typeof raw.soulPath === 'string' ? raw.soulPath : '';
	const createdAt = typeof raw.createdAt === 'number' ? raw.createdAt : Date.now();
	if (!id || !name || !soulPath) return null;
	return { id, name, avatarPath, soulPath, createdAt };
}

export function normalizeCustomPersonas(value: unknown): CustomPersona[] {
	if (!Array.isArray(value)) return [];
	const result: CustomPersona[] = [];
	for (const item of value) {
		const persona = normalizeCustomPersona(item);
		if (persona) result.push(persona);
	}
	return result;
}

/**
 * Normalizes a `personaId` given the known custom personas. Falls back to the
 * default builtin if the id doesn't match a builtin or a known custom persona.
 */
export function normalizePersonaId(
	value: unknown,
	customPersonas: CustomPersona[] | undefined
): string {
	if (isBuiltinPersonaId(value)) return value;
	if (typeof value === 'string' && (customPersonas ?? []).some((p) => p.id === value)) {
		return value;
	}
	return 'minako';
}
