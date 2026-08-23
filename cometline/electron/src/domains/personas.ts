import type { App, BrowserWindow, Tray } from 'electron';

import type { CustomPersona, ProviderSettings } from '../../../src/lib/types.js';

type PersonaFileSystem = Pick<
	typeof import('node:fs'),
	'existsSync' | 'mkdirSync' | 'readFileSync' | 'statSync' | 'writeFileSync' | 'promises'
>;
type PersonaPath = Pick<
	typeof import('node:path'),
	'dirname' | 'extname' | 'join' | 'resolve' | 'sep'
>;
type NativeImageService = Pick<typeof import('electron').nativeImage, 'createFromPath'>;

const BUILTIN_PERSONA_IDS = new Set(['minako', 'souma']);
const PERSONA_IMAGE_MIME_BY_EXT: Record<string, string> = {
	'.png': 'image/png',
	'.jpg': 'image/jpeg',
	'.jpeg': 'image/jpeg',
	'.webp': 'image/webp'
};
const PERSONA_AVATAR_MAX_BYTES = 20 * 1024 * 1024;

export type ReadPersonaSoulResult = { ok: true; content: string } | { ok: false; error: string };
export type ReadPersonaAvatarResult = { ok: true; dataUrl: string } | { ok: false; error: string };
export type SaveCustomPersonaResult =
	| { ok: true; persona: CustomPersona }
	| { ok: false; error: string };
export type DeleteCustomPersonaResult = { ok: true } | { ok: false; error: string };

export interface PersonaDependencies {
	fs: PersonaFileSystem;
	path: PersonaPath;
	homedir: () => string;
	environment: Record<string, string | undefined>;
	platform: NodeJS.Platform;
	app: Pick<App, 'getAppPath' | 'isPackaged' | 'dock'>;
	resourcesPath: string;
	nativeImage: NativeImageService;
	runtimeDirectory: string;
	getMainWindow: () => BrowserWindow | null;
	getMiniWindow: () => BrowserWindow | null;
	getTray: () => Tray | null;
	getSettings: () => ProviderSettings;
	writeSettings: (settings: ProviderSettings) => ProviderSettings;
	reloadCometMind: () => Promise<unknown>;
	broadcastProviderSettingsChanged: (settings: ProviderSettings) => void;
	broadcastPersonaAvatarChanged: (personaId: string) => void;
	now?: () => number;
}

export function normalizePersonaSlug(value: unknown) {
	return String(value || '')
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 48);
}

export function decodePersonaAvatarDataUrl(dataUrl: unknown) {
	const match = String(dataUrl || '').match(/^data:(image\/(?:png|jpeg|webp));base64,([A-Za-z0-9+/=]+)$/);
	if (!match) return null;
	const ext = match[1] === 'image/jpeg' ? '.jpg' : match[1] === 'image/webp' ? '.webp' : '.png';
	const buffer = Buffer.from(match[2], 'base64');
	if (buffer.length > PERSONA_AVATAR_MAX_BYTES) {
		throw new Error('Avatar image exceeds 20 MB limit');
	}
	return { ext, buffer };
}

export function nextCustomPersonaId(
	requestedId: unknown,
	name: unknown,
	existingPersonas: Pick<CustomPersona, 'id'>[],
	now: () => number = Date.now
) {
	const existing = new Set(existingPersonas.map((persona) => persona.id));
	const base = normalizePersonaSlug(requestedId) || normalizePersonaSlug(name) || 'persona';
	if (requestedId || !existing.has(base)) return base;
	for (let i = 2; i < 1000; i += 1) {
		const candidate = `${base}-${i}`;
		if (!existing.has(candidate)) return candidate;
	}
	return `${base}-${now()}`;
}

/** Owns persona persistence, validation, and product-icon application. */
export function createPersonas(dependencies: PersonaDependencies) {
	const { app, fs, nativeImage, path } = dependencies;
	const now = dependencies.now ?? Date.now;

	function migratePersonaIdFromIconVariant(iconVariant: unknown) {
		return iconVariant === 'man' ? 'souma' : 'minako';
	}

	function isBuiltinPersonaId(personaId: unknown) {
		return BUILTIN_PERSONA_IDS.has(String(personaId || ''));
	}

	function expandHomePath(filePath: unknown) {
		const clean = String(filePath || '').trim();
		if (clean === '~') return dependencies.homedir();
		if (clean.startsWith(`~${path.sep}`) || clean.startsWith('~/')) {
			return path.join(dependencies.homedir(), clean.slice(2));
		}
		return clean;
	}

	function getPersonasDir() {
		const directory = path.join(dependencies.homedir(), '.cometmind', 'personas');
		if (!fs.existsSync(directory)) fs.mkdirSync(directory, { recursive: true });
		return directory;
	}

	function getPersonaDir(id: unknown) {
		const slug = normalizePersonaSlug(id);
		if (!slug) return '';
		const root = getPersonasDir();
		const directory = path.resolve(root, slug);
		if (directory !== root && directory.startsWith(root + path.sep)) return directory;
		return '';
	}

	function builtinSoulFilename(personaId: unknown) {
		return personaId === 'souma' ? 'SOUL_MAN.md' : 'SOUL.md';
	}

	function resolveBuiltinSoulPath(personaId: unknown = 'minako') {
		const filename = builtinSoulFilename(personaId);
		if (app.isPackaged) return path.join(dependencies.resourcesPath, filename);
		return path.join(dependencies.runtimeDirectory, '..', filename);
	}

	function normalizeCustomPersonaEntry(value: unknown): CustomPersona | null {
		if (!value || typeof value !== 'object') return null;
		const entry = value as Partial<CustomPersona>;
		const id = normalizePersonaSlug(entry.id);
		const name = String(entry.name || '').trim();
		const soulPath = path.resolve(expandHomePath(entry.soulPath));
		if (!id || !name || !soulPath) return null;
		const avatarPath = String(entry.avatarPath || '').trim()
			? path.resolve(expandHomePath(entry.avatarPath))
			: '';
		return {
			id,
			name,
			avatarPath,
			soulPath,
			createdAt: Number.isFinite(entry.createdAt) ? Number(entry.createdAt) : now()
		};
	}

	function customPersonasFromSettings(settings: unknown) {
		const custom = (settings as ProviderSettings | undefined)?.app?.personas?.custom;
		if (!Array.isArray(custom)) return [];
		return custom.map(normalizeCustomPersonaEntry).filter((persona): persona is CustomPersona => Boolean(persona));
	}

	function findCustomPersona(settings: unknown, personaId: unknown) {
		const id = normalizePersonaSlug(personaId);
		return customPersonasFromSettings(settings).find((persona) => persona.id === id) ?? null;
	}

	function readSavedPersonaId(saved: unknown) {
		const settings = saved as ProviderSettings | undefined;
		const requested = String(settings?.app?.personaId || '').trim();
		if (isBuiltinPersonaId(requested) || findCustomPersona(saved, requested)) return requested;
		return migratePersonaIdFromIconVariant((saved as { app?: { iconVariant?: unknown } })?.app?.iconVariant);
	}

	function resolveNextPersonaId(settings: Partial<ProviderSettings>, current: ProviderSettings) {
		const merged = { app: { ...(current.app ?? {}), ...(settings.app ?? {}) } };
		const requested = String(settings.app?.personaId || '').trim();
		if (isBuiltinPersonaId(requested) || findCustomPersona(merged, requested)) return requested;
		return readSavedPersonaId(current);
	}

	function resolveSystemPromptPath(personaId: unknown = 'minako', settings: unknown = undefined) {
		if (dependencies.environment.COMETMIND_SYSTEM_PROMPT_PATH) {
			return path.resolve(dependencies.environment.COMETMIND_SYSTEM_PROMPT_PATH);
		}
		const customPersona = findCustomPersona(settings, personaId);
		if (customPersona?.soulPath) return path.resolve(expandHomePath(customPersona.soulPath));
		return resolveBuiltinSoulPath(personaId);
	}

	function resolveTrayResourcePath(filename: string) {
		if (app.isPackaged) return path.join(dependencies.resourcesPath, filename);
		return path.join(app.getAppPath(), 'buildResources', filename);
	}

	function loadMacOSTrayImage(baseFilename: string, { template = false } = {}) {
		const imagePath = resolveTrayResourcePath(baseFilename);
		if (!fs.existsSync(imagePath)) return null;
		const image = nativeImage.createFromPath(imagePath);
		if (image.isEmpty()) return null;
		const size = image.getSize();
		const scaleFactors = image.getScaleFactors();
		if (scaleFactors.length === 1 && scaleFactors[0] === 1 && size.width > 18) {
			const resized = image.resize({ width: 16, height: 16, quality: 'best' });
			if (resized.isEmpty()) return null;
			if (template) resized.setTemplateImage(true);
			return resized;
		}
		if (template) image.setTemplateImage(true);
		return image;
	}

	function resolveTrayIconCandidates() {
		if (dependencies.platform === 'darwin') {
			return ['trayIcon.png', 'trayTemplate.png'];
		}
		return ['trayIcon.png', 'icon.png'];
	}

	function resolveTrayIcon() {
		const candidates = resolveTrayIconCandidates();
		for (const filename of candidates) {
			const resourcePath = resolveTrayResourcePath(filename);
			if (!fs.existsSync(resourcePath)) continue;
			if (dependencies.platform === 'darwin') {
				const isTemplateAsset = filename === 'trayTemplate.png';
				const image = loadMacOSTrayImage(filename, { template: isTemplateAsset });
				if (image) {
					if (!app.isPackaged) {
						console.log('[tray] Using', resourcePath, image.getSize(), image.getScaleFactors());
					}
					return image;
				}
				continue;
			}
			const source = nativeImage.createFromPath(resourcePath);
			if (source.isEmpty()) continue;
			return source.resize({ width: 18, height: 18, quality: 'best' });
		}
		const checked = candidates.map((name) => resolveTrayResourcePath(name));
		console.warn('[tray] No tray icon found; checked:', checked.join(', '));
		return null;
	}

	function resolveAppIconPaths() {
		if (app.isPackaged) return [path.join(dependencies.resourcesPath, 'icon.png')];
		return [
			path.join(app.getAppPath(), 'static', 'app_icon.png'),
			path.join(dependencies.runtimeDirectory, '..', 'static', 'app_icon.png'),
			path.join(app.getAppPath(), 'buildResources', 'icon.png'),
			path.join(dependencies.runtimeDirectory, '..', 'buildResources', 'icon.png')
		];
	}

	function getAppIconPath() {
		return resolveAppIconPaths().find((candidate) => fs.existsSync(candidate));
	}

	function getAppIconImage() {
		const iconPath = getAppIconPath();
		if (!iconPath) return null;
		const image = nativeImage.createFromPath(iconPath);
		return image.isEmpty() ? null : image;
	}

	function resolveTrayImageSource() {
		const trayIconPath = resolveTrayResourcePath('trayIcon.png');
		if (fs.existsSync(trayIconPath)) return trayIconPath;
		return resolveTrayIcon();
	}

	function applyProductIcon() {
		const image = getAppIconImage();
		if (!image) {
			console.warn('[icon] No product app icon found');
			return;
		}
		if (dependencies.platform === 'darwin') app.dock?.setIcon(image);
		const mainWindow = dependencies.getMainWindow();
		if (mainWindow && !mainWindow.isDestroyed()) mainWindow.setIcon(image);
		const miniWindow = dependencies.getMiniWindow();
		if (miniWindow && !miniWindow.isDestroyed()) miniWindow.setIcon(image);
		const tray = dependencies.getTray();
		if (!tray) return;
		const trayImageSource = resolveTrayImageSource();
		if (typeof trayImageSource === 'string') {
			tray.setImage(trayImageSource);
			return;
		}
		if (trayImageSource) tray.setImage(trayImageSource);
	}

	async function readPersonaSoul(personaId: unknown): Promise<ReadPersonaSoulResult> {
		const settings = dependencies.getSettings();
		const customPersona = findCustomPersona(settings, personaId);
		const soulPath = customPersona?.soulPath ?? resolveBuiltinSoulPath(personaId);
		try {
			const stat = await fs.promises.stat(soulPath);
			if (!stat.isFile()) return { ok: false, error: 'SOUL file is not a file' };
			const content = await fs.promises.readFile(soulPath, 'utf8');
			return { ok: true, content };
		} catch {
			return { ok: false, error: 'SOUL file not found' };
		}
	}

	async function readPersonaAvatar(id: unknown): Promise<ReadPersonaAvatarResult> {
		const settings = dependencies.getSettings();
		const customPersona = findCustomPersona(settings, id);
		if (!customPersona?.avatarPath) return { ok: false, error: 'Avatar image not found' };
		const avatarPath = path.resolve(expandHomePath(customPersona.avatarPath));
		let stat;
		try {
			stat = await fs.promises.stat(avatarPath);
		} catch {
			return { ok: false, error: 'Avatar image not found' };
		}
		if (!stat.isFile()) return { ok: false, error: 'Avatar image is not a file' };
		if (stat.size > PERSONA_AVATAR_MAX_BYTES) {
			return { ok: false, error: 'Avatar image exceeds 20 MB limit' };
		}
		const ext = path.extname(avatarPath).toLowerCase();
		const mimeType = PERSONA_IMAGE_MIME_BY_EXT[ext];
		if (!mimeType) return { ok: false, error: 'Unsupported avatar image type' };
		const buffer = await fs.promises.readFile(avatarPath);
		return { ok: true, dataUrl: `data:${mimeType};base64,${buffer.toString('base64')}` };
	}

	async function saveCustomPersona(payload: {
		id?: unknown;
		name?: unknown;
		soulMarkdown?: unknown;
		avatarDataUrl?: unknown;
	} = {}): Promise<SaveCustomPersonaResult> {
		const name = String(payload.name || '').trim();
		const soulMarkdown = String(payload.soulMarkdown || '').trim();
		if (!name) return { ok: false, error: 'Persona name is required.' };
		if (!soulMarkdown) return { ok: false, error: 'SOUL.md content is required.' };
		const settings = dependencies.getSettings();
		const customPersonas = customPersonasFromSettings(settings);
		const id = nextCustomPersonaId(payload.id, name, customPersonas, now);
		const personaDir = getPersonaDir(id);
		if (!personaDir) return { ok: false, error: 'Invalid persona id.' };
		await fs.promises.mkdir(personaDir, { recursive: true });
		const existing = customPersonas.find((persona) => persona.id === id);
		const soulPath = path.join(personaDir, 'SOUL.md');
		await fs.promises.writeFile(soulPath, `${soulMarkdown}\n`, { mode: 0o600 });
		let avatarPath = existing?.avatarPath ?? '';
		if (payload.avatarDataUrl) {
			let decoded: ReturnType<typeof decodePersonaAvatarDataUrl>;
			try {
				decoded = decodePersonaAvatarDataUrl(payload.avatarDataUrl);
			} catch (error) {
				return { ok: false, error: error instanceof Error ? error.message : 'Invalid avatar image.' };
			}
			if (!decoded) return { ok: false, error: 'Avatar image must be PNG, JPEG, or WebP.' };
			avatarPath = path.join(personaDir, `avatar${decoded.ext}`);
			await fs.promises.writeFile(avatarPath, decoded.buffer, { mode: 0o600 });
		}
		const persona: CustomPersona = {
			id,
			name,
			avatarPath,
			soulPath,
			createdAt: existing?.createdAt ?? now()
		};
		const nextCustomPersonas = [...customPersonas.filter((item) => item.id !== id), persona];
		settings.app = {
			...settings.app,
			personaId: id,
			personas: { custom: nextCustomPersonas }
		};
		const saved = dependencies.writeSettings(settings);
		await dependencies.reloadCometMind();
		dependencies.broadcastProviderSettingsChanged(saved);
		if (payload.avatarDataUrl) dependencies.broadcastPersonaAvatarChanged(persona.id);
		return { ok: true, persona };
	}

	async function deleteCustomPersona(id: unknown): Promise<DeleteCustomPersonaResult> {
		const cleanId = normalizePersonaSlug(id);
		if (!cleanId) return { ok: false, error: 'Invalid persona id.' };
		const settings = dependencies.getSettings();
		const customPersonas = customPersonasFromSettings(settings);
		const existing = customPersonas.find((persona) => persona.id === cleanId);
		if (!existing) return { ok: false, error: 'Persona not found.' };
		settings.app = {
			...settings.app,
			personaId: settings.app?.personaId === cleanId ? 'minako' : settings.app?.personaId,
			personas: { custom: customPersonas.filter((persona) => persona.id !== cleanId) }
		};
		const saved = dependencies.writeSettings(settings);
		const personaDir = getPersonaDir(cleanId);
		if (personaDir) await fs.promises.rm(personaDir, { recursive: true, force: true });
		await dependencies.reloadCometMind();
		dependencies.broadcastProviderSettingsChanged(saved);
		return { ok: true };
	}

	return {
		applyProductIcon,
		customPersonasFromSettings,
		deleteCustomPersona,
		getAppIconImage,
		readPersonaAvatar,
		readPersonaSoul,
		readSavedPersonaId,
		resolveNextPersonaId,
		resolveSystemPromptPath,
		resolveTrayIcon,
		resolveTrayResourcePath,
		saveCustomPersona
	};
}
