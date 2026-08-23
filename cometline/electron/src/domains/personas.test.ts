import { describe, expect, it, vi } from 'vitest';

import {
	createPersonas,
	decodePersonaAvatarDataUrl,
	nextCustomPersonaId,
	normalizePersonaSlug
} from './personas.js';

describe('persona input validation', () => {
	it('normalizes custom IDs into traversal-safe slugs', () => {
		expect(normalizePersonaSlug(' ../My Persona/../../ ')).toBe('my-persona');
		expect(normalizePersonaSlug('...')).toBe('');
		expect(normalizePersonaSlug('a'.repeat(60))).toHaveLength(48);
	});

	it('accepts only PNG, JPEG, and WebP avatar data URLs', () => {
		expect(decodePersonaAvatarDataUrl('data:image/png;base64,YQ==')).toMatchObject({
			ext: '.png',
			buffer: Buffer.from('a')
		});
		expect(decodePersonaAvatarDataUrl('data:image/jpeg;base64,YQ==')).toMatchObject({ ext: '.jpg' });
		expect(decodePersonaAvatarDataUrl('data:image/webp;base64,YQ==')).toMatchObject({ ext: '.webp' });
		expect(decodePersonaAvatarDataUrl('data:image/jpg;base64,YQ==')).toBeNull();
		expect(decodePersonaAvatarDataUrl('data:image/gif;base64,YQ==')).toBeNull();
		expect(decodePersonaAvatarDataUrl('data:image/png;base64,not valid')).toBeNull();
	});

	it('rejects avatars larger than the 20 MB limit', () => {
		const payload = Buffer.alloc(20 * 1024 * 1024 + 1).toString('base64');
		expect(() => decodePersonaAvatarDataUrl(`data:image/png;base64,${payload}`)).toThrow(
			'Avatar image exceeds 20 MB limit'
		);
	});

	it('creates a stable, collision-free ID for an unnamed custom persona', () => {
		expect(
			nextCustomPersonaId('', 'My Persona', [{ id: 'my-persona' }], () => 1234)
		).toBe('my-persona-2');
		expect(nextCustomPersonaId('preferred', 'My Persona', [{ id: 'preferred' }])).toBe('preferred');
	});
});

describe('product icons stay persona-independent', () => {
	it('always applies the product Dock and tray icons', () => {
		const productIcon = { isEmpty: () => false };
		const trayIcon = { isEmpty: () => false };
		const dock = { setIcon: vi.fn() };
		const tray = { setImage: vi.fn() };
		const personas = createPersonas({
			fs: {
				existsSync: (candidate: string) =>
					candidate.endsWith('/icon.png') || candidate.endsWith('/trayIcon.png'),
				mkdirSync: vi.fn(),
				readFileSync: vi.fn(),
				statSync: vi.fn(),
				writeFileSync: vi.fn(),
				promises: {}
			} as never,
			path: {
				dirname: (value: string) => value,
				extname: (value: string) => {
					const index = value.lastIndexOf('.');
					return index >= 0 ? value.slice(index) : '';
				},
				join: (...parts: string[]) => parts.join('/'),
				resolve: (...parts: string[]) => parts.join('/'),
				sep: '/'
			},
			homedir: () => '/home',
			environment: {},
			platform: 'darwin',
			app: {
				getAppPath: () => '/app',
				isPackaged: true,
				dock
			},
			resourcesPath: '/resources',
			nativeImage: {
				createFromPath: vi.fn((candidate: string) =>
					candidate.endsWith('/icon.png') ? productIcon : { isEmpty: () => true }
				)
			},
			runtimeDirectory: '/runtime',
			getMainWindow: () => null,
			getMiniWindow: () => null,
			getTray: () => tray as never,
			getSettings: () => ({ app: { personaId: 'souma' } }) as never,
			writeSettings: vi.fn(),
			reloadCometMind: vi.fn(),
			broadcastProviderSettingsChanged: vi.fn(),
			broadcastPersonaAvatarChanged: vi.fn()
		});

		expect(personas.getAppIconImage()).toBe(productIcon);
		expect(personas.resolveTrayResourcePath('trayIcon.png')).toBe('/resources/trayIcon.png');
		personas.applyProductIcon();
		expect(dock.setIcon).toHaveBeenCalledWith(productIcon);
		expect(tray.setImage).toHaveBeenCalledWith('/resources/trayIcon.png');
		expect(trayIcon.isEmpty()).toBe(false);
	});
});
