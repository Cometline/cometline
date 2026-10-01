import type { DeleteCustomPersonaResult, SaveCustomPersonaResult } from '$lib/electron-api';
import { personaAvatarCache } from '$lib/personas/avatar-cache.svelte';

export interface CustomPersonaLike {
	id: string;
	name: string;
	avatarPath: string;
	soulPath: string;
	createdAt: number;
}

export function createSettingsPanelPersonaEditor(deps: {
	saveCustomPersona: (payload: {
		id?: string;
		name: string;
		soulMarkdown: string;
		avatarDataUrl?: string;
	}) => Promise<SaveCustomPersonaResult>;
	deleteCustomPersona: (id: string) => Promise<DeleteCustomPersonaResult>;
}) {
	let customPersonas = $state<CustomPersonaLike[]>([]);
	let soulPreviewOpen = $state(false);
	let soulPreviewText = $state('');
	let soulPreviewLabel = $state('');
	let soulPreviewCopied = $state(false);
	let personaEditorOpen = $state(false);
	let personaEditorId = $state<string | undefined>(undefined);
	let personaEditorName = $state('');
	let personaEditorSoul = $state('');
	let personaEditorAvatarDataUrl = $state<string | undefined>(undefined);
	let personaEditorError = $state('');
	let personaEditorBusy = $state(false);

	async function refreshCustomPersonas() {
		if (!window.electronAPI?.listCustomPersonas) return;
		customPersonas = await window.electronAPI.listCustomPersonas();
	}

	async function openBuiltinSoulPreview(personaId: string, label: string) {
		soulPreviewLabel = label;
		soulPreviewText = 'Loading…';
		soulPreviewOpen = true;
		soulPreviewCopied = false;
		if (!window.electronAPI?.readBuiltinSoul) {
			soulPreviewText = 'SOUL preview is only available in the desktop app.';
			return;
		}
		const result = await window.electronAPI.readBuiltinSoul(personaId);
		soulPreviewText = result.ok ? result.content : `Could not load SOUL: ${result.error}`;
	}

	async function copySoulPreview() {
		try {
			await navigator.clipboard.writeText(soulPreviewText);
			soulPreviewCopied = true;
			setTimeout(() => (soulPreviewCopied = false), 1500);
		} catch {
			// ignore clipboard failures
		}
	}

	function closeSoulPreview() {
		soulPreviewOpen = false;
	}

	function openPersonaEditorForCreate() {
		personaEditorId = undefined;
		personaEditorName = '';
		personaEditorSoul = '';
		personaEditorAvatarDataUrl = undefined;
		personaEditorError = '';
		personaEditorOpen = true;
	}

	async function openPersonaEditorForEdit(persona: CustomPersonaLike) {
		personaEditorId = persona.id;
		personaEditorName = persona.name;
		personaEditorAvatarDataUrl = undefined;
		personaEditorError = '';
		personaEditorSoul = '';
		if (window.electronAPI?.readPersonaAvatar) {
			const avatar = await window.electronAPI.readPersonaAvatar(persona.id);
			if (avatar.ok) personaEditorAvatarDataUrl = avatar.dataUrl;
		}
		personaEditorOpen = true;
		// Load current SOUL text for editing by reading it back via readBuiltinSoul-style path.
		if (window.electronAPI?.readBuiltinSoul) {
			const soul = await window.electronAPI.readBuiltinSoul(persona.id);
			if (soul.ok) personaEditorSoul = soul.content;
		}
	}

	function closePersonaEditor() {
		personaEditorOpen = false;
	}

	function onPersonaAvatarFileChange(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		const reader = new FileReader();
		reader.onload = () => {
			personaEditorAvatarDataUrl = String(reader.result);
		};
		reader.readAsDataURL(file);
	}

	async function submitPersonaEditor() {
		personaEditorError = '';
		if (!personaEditorName.trim()) {
			personaEditorError = 'Give your persona a name.';
			return;
		}
		if (!personaEditorSoul.trim()) {
			personaEditorError = 'SOUL content cannot be empty.';
			return;
		}
		personaEditorBusy = true;
		try {
			const result = await deps.saveCustomPersona({
				id: personaEditorId,
				name: personaEditorName.trim(),
				soulMarkdown: personaEditorSoul,
				avatarDataUrl: personaEditorAvatarDataUrl
			});
			if (!result.ok) {
				personaEditorError = result.error;
				return;
			}
			personaEditorOpen = false;
			await refreshCustomPersonas();
			personaAvatarCache.invalidate(result.persona.id);
		} finally {
			personaEditorBusy = false;
		}
	}

	async function removeCustomPersona(id: string) {
		const result = await deps.deleteCustomPersona(id);
		if (result.ok) {
			await refreshCustomPersonas();
		}
	}

	return {
		get customPersonas() {
			return customPersonas;
		},
		get soulPreviewOpen() {
			return soulPreviewOpen;
		},
		get soulPreviewText() {
			return soulPreviewText;
		},
		get soulPreviewLabel() {
			return soulPreviewLabel;
		},
		get soulPreviewCopied() {
			return soulPreviewCopied;
		},
		get personaEditorOpen() {
			return personaEditorOpen;
		},
		get personaEditorId() {
			return personaEditorId;
		},
		get personaEditorName() {
			return personaEditorName;
		},
		set personaEditorName(value: string) {
			personaEditorName = value;
		},
		get personaEditorSoul() {
			return personaEditorSoul;
		},
		set personaEditorSoul(value: string) {
			personaEditorSoul = value;
		},
		get personaEditorAvatarDataUrl() {
			return personaEditorAvatarDataUrl;
		},
		get personaEditorError() {
			return personaEditorError;
		},
		get personaEditorBusy() {
			return personaEditorBusy;
		},
		refreshCustomPersonas,
		openBuiltinSoulPreview,
		copySoulPreview,
		closeSoulPreview,
		openPersonaEditorForCreate,
		openPersonaEditorForEdit,
		closePersonaEditor,
		onPersonaAvatarFileChange,
		submitPersonaEditor,
		removeCustomPersona
	};
}
