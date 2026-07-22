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
type NativeImageService = Pick<
	typeof import('electron').nativeImage,
	'createFromBitmap' | 'createFromPath'
>;

const BUILTIN_PERSONA_IDS = new Set(['minako', 'souma']);
const PERSONA_IMAGE_MIME_BY_EXT: Record<string, string> = {
	'.png': 'image/png',
	'.jpg': 'image/jpeg',
	'.jpeg': 'image/jpeg',
	'.webp': 'image/webp'
};
const PERSONA_AVATAR_MAX_BYTES = 2 * 1024 * 1024;
const PERSONA_APP_ICON_SIZE = 1024;
const PERSONA_APP_ICON_RADIUS = 224;
const PERSONA_APP_ICON_ARTWORK_SCALE = 0.8125;

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
		throw new Error('Avatar image exceeds 2 MB limit');
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

/** Owns persona persistence, validation, native icon generation, and application. */
export function createPersonas(dependencies: PersonaDependencies) {
	const { app, fs, nativeImage, path } = dependencies;
	const now = dependencies.now ?? Date.now;

	function migratePersonaIdFromIconVariant(iconVariant: unknown) {
		return iconVariant === 'man' ? 'souma' : 'minako';
	}

	function builtinPersonaToIconVariant(personaId: unknown) {
		return personaId === 'souma' ? 'man' : 'default';
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

	function getPersonaId() {
		return readSavedPersonaId(dependencies.getSettings());
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

	function resolveTrayIconCandidates(variant = 'default') {
		const trayIcon = variant === 'man' ? 'trayIcon_man.png' : 'trayIcon.png';
		if (dependencies.platform === 'darwin') {
			const candidates = [trayIcon];
			if (variant === 'man') candidates.push('trayIcon.png');
			candidates.push('trayTemplate.png');
			return candidates;
		}
		return [trayIcon, 'icon.png'];
	}

	function resolveTrayIcon(variant = 'default') {
		const candidates = resolveTrayIconCandidates(variant);
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

	function customPersonaAppIconPath(personaId: unknown) {
		const directory = getPersonaDir(personaId);
		return directory ? path.join(directory, 'app_icon.png') : '';
	}

	function resolveAppIconPaths(personaId: unknown = 'minako', settings: unknown = undefined) {
		const customPersona = findCustomPersona(settings ?? dependencies.getSettings(), personaId);
		if (customPersona) {
			const appIconPath = customPersonaAppIconPath(customPersona.id);
			return appIconPath ? [appIconPath] : [];
		}
		const variant = builtinPersonaToIconVariant(personaId);
		if (variant === 'man') {
			if (app.isPackaged) return [path.join(dependencies.resourcesPath, 'app_icon_man.png')];
			return [
				path.join(app.getAppPath(), 'static', 'app_icon_man.png'),
				path.join(dependencies.runtimeDirectory, '..', 'static', 'app_icon_man.png')
			];
		}
		if (app.isPackaged) return [path.join(dependencies.resourcesPath, 'icon.png')];
		return [
			path.join(app.getAppPath(), 'static', 'app_icon.png'),
			path.join(dependencies.runtimeDirectory, '..', 'static', 'app_icon.png'),
			path.join(dependencies.runtimeDirectory, '..', 'buildResources', 'icon.png')
		];
	}

	function getAppIconPath(personaId: unknown = getPersonaId(), settings: unknown = undefined) {
		return resolveAppIconPaths(personaId, settings).find((candidate) => fs.existsSync(candidate));
	}

	function roundedRectCoverage(x: number, y: number, width: number, height: number, radius: number) {
		const px = x + 0.5;
		const py = y + 0.5;
		if (px >= radius && px <= width - radius) return 1;
		if (py >= radius && py <= height - radius) return 1;
		const cx = px < radius ? radius : width - radius;
		const cy = py < radius ? radius : height - radius;
		const dist = Math.hypot(px - cx, py - cy);
		if (dist <= radius - 0.5) return 1;
		if (dist >= radius + 0.5) return 0;
		return radius + 0.5 - dist;
	}

	function createCustomPersonaAppIcon(customPersona: Pick<CustomPersona, 'avatarPath'>) {
		if (!customPersona.avatarPath || !fs.existsSync(customPersona.avatarPath)) return null;
		const ext = path.extname(customPersona.avatarPath).toLowerCase();
		if (!PERSONA_IMAGE_MIME_BY_EXT[ext]) return null;
		let source = nativeImage.createFromPath(path.resolve(customPersona.avatarPath));
		if (source.isEmpty()) return null;
		const srcSize = source.getSize();
		if (srcSize.width !== srcSize.height) {
			const side = Math.min(srcSize.width, srcSize.height);
			source = source.crop({
				x: Math.round((srcSize.width - side) / 2),
				y: Math.round((srcSize.height - side) / 2),
				width: side,
				height: side
			});
		}
		const size = PERSONA_APP_ICON_SIZE;
		const artwork = Math.round(size * PERSONA_APP_ICON_ARTWORK_SCALE);
		const inset = Math.round((size - artwork) / 2);
		const radius = Math.round(PERSONA_APP_ICON_RADIUS * PERSONA_APP_ICON_ARTWORK_SCALE);
		const scaled = source.resize({ width: artwork, height: artwork, quality: 'best' });
		const sourceBitmap = Buffer.from(scaled.toBitmap());
		if (sourceBitmap.length < artwork * artwork * 4) return null;
		const canvas = Buffer.alloc(size * size * 4);
		for (let y = 0; y < artwork; y++) {
			for (let x = 0; x < artwork; x++) {
				const coverage = roundedRectCoverage(x, y, artwork, artwork, radius);
				if (coverage <= 0) continue;
				const sourceOffset = (y * artwork + x) * 4;
				const alpha = sourceBitmap[sourceOffset + 3] / 255;
				const destinationOffset = ((y + inset) * size + (x + inset)) * 4;
				canvas[destinationOffset] = Math.round(sourceBitmap[sourceOffset] * alpha + 255 * (1 - alpha));
				canvas[destinationOffset + 1] = Math.round(sourceBitmap[sourceOffset + 1] * alpha + 255 * (1 - alpha));
				canvas[destinationOffset + 2] = Math.round(sourceBitmap[sourceOffset + 2] * alpha + 255 * (1 - alpha));
				canvas[destinationOffset + 3] = Math.round(255 * coverage);
			}
		}
		const image = nativeImage.createFromBitmap(canvas, { width: size, height: size });
		return image.isEmpty() ? null : image;
	}

	function generatePersonaAppIconPng(avatarPath: string, outputPath: string) {
		if (!avatarPath || !fs.existsSync(avatarPath) || !outputPath) return false;
		const image = createCustomPersonaAppIcon({ avatarPath });
		if (!image) return false;
		try {
			fs.mkdirSync(path.dirname(outputPath), { recursive: true });
			fs.writeFileSync(outputPath, image.toPNG());
			return true;
		} catch {
			return false;
		}
	}

	function ensureCustomPersonaAppIcon(customPersona: CustomPersona) {
		if (!customPersona.avatarPath || !fs.existsSync(customPersona.avatarPath)) return null;
		const iconPath = customPersonaAppIconPath(customPersona.id);
		if (!iconPath) return createCustomPersonaAppIcon(customPersona);
		let avatarMtime = 0;
		let iconMtime = 0;
		try {
			avatarMtime = fs.statSync(customPersona.avatarPath).mtimeMs;
			if (fs.existsSync(iconPath)) iconMtime = fs.statSync(iconPath).mtimeMs;
		} catch {
			return createCustomPersonaAppIcon(customPersona);
		}
		if (!fs.existsSync(iconPath) || iconMtime < avatarMtime) {
			generatePersonaAppIconPng(customPersona.avatarPath, iconPath);
		}
		if (fs.existsSync(iconPath)) {
			const cached = nativeImage.createFromPath(iconPath);
			if (!cached.isEmpty()) return cached;
		}
		return createCustomPersonaAppIcon(customPersona);
	}

	function getAppIconImage(personaId: unknown = getPersonaId(), settings: unknown = undefined) {
		const resolvedSettings = settings ?? dependencies.getSettings();
		const customPersona = findCustomPersona(resolvedSettings, personaId);
		if (customPersona) {
			const customIcon = ensureCustomPersonaAppIcon(customPersona);
			if (customIcon) return customIcon;
		}
		const iconPath = getAppIconPath(personaId, resolvedSettings);
		if (!iconPath) return null;
		const image = nativeImage.createFromPath(iconPath);
		return image.isEmpty() ? null : image;
	}

	function resolveTrayImageSource(personaId: unknown = getPersonaId(), settings: unknown = undefined) {
		const customPersona = findCustomPersona(settings ?? dependencies.getSettings(), personaId);
		if (customPersona?.avatarPath && fs.existsSync(customPersona.avatarPath)) {
			const image = ensureCustomPersonaAppIcon(customPersona);
			if (image && !image.isEmpty()) return image.resize({ width: 18, height: 18, quality: 'best' });
		}
		const variant = builtinPersonaToIconVariant(personaId);
		const trayIconPath = resolveTrayResourcePath(
			variant === 'man' ? 'trayIcon_man.png' : 'trayIcon.png'
		);
		if (fs.existsSync(trayIconPath)) return trayIconPath;
		const fallbackTrayPath = resolveTrayResourcePath('trayIcon.png');
		if (fs.existsSync(fallbackTrayPath)) return fallbackTrayPath;
		return resolveTrayIcon(variant);
	}

	function applyPersona(personaId: unknown = getPersonaId(), settings: unknown = undefined) {
		const image = getAppIconImage(personaId, settings);
		if (!image) {
			console.warn('[icon] No app icon found for persona', personaId);
			return;
		}
		if (dependencies.platform === 'darwin') app.dock?.setIcon(image);
		const mainWindow = dependencies.getMainWindow();
		if (mainWindow && !mainWindow.isDestroyed()) mainWindow.setIcon(image);
		const miniWindow = dependencies.getMiniWindow();
		if (miniWindow && !miniWindow.isDestroyed()) miniWindow.setIcon(image);
		const tray = dependencies.getTray();
		if (!tray) return;
		const trayImageSource = resolveTrayImageSource(personaId, settings);
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
			return { ok: false, error: 'Avatar image exceeds 2 MB limit' };
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
		generatePersonaAppIconPng(persona.avatarPath, customPersonaAppIconPath(persona.id));
		await dependencies.reloadCometMind();
		applyPersona(saved.app?.personaId, saved);
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
		applyPersona(saved.app?.personaId, saved);
		dependencies.broadcastProviderSettingsChanged(saved);
		return { ok: true };
	}

	return {
		applyPersona,
		builtinPersonaToIconVariant,
		customPersonasFromSettings,
		deleteCustomPersona,
		getAppIconImage,
		getPersonaId,
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
