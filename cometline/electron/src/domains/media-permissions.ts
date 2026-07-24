import { desktopCapturer, shell, systemPreferences } from 'electron';
import os from 'node:os';

const MACOS_SCREEN_RECORDING_SETTINGS_URL =
	'x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture';

export type ScreenCaptureAccessStatus =
	| 'granted'
	| 'denied'
	| 'not-determined'
	| 'restricted'
	| 'unknown'
	| 'unsupported';

export interface ScreenCaptureAccessState {
	preferred: boolean;
	status: ScreenCaptureAccessStatus;
	openedSettings?: boolean;
	message?: string;
}

function isDarwin() {
	return process.platform === 'darwin';
}

function readScreenAccessStatus(): ScreenCaptureAccessStatus {
	if (!isDarwin()) return 'unsupported';
	try {
		const status = systemPreferences.getMediaAccessStatus('screen');
		switch (status) {
			case 'granted':
			case 'denied':
			case 'not-determined':
			case 'restricted':
				return status;
			default:
				return 'unknown';
		}
	} catch {
		return 'unknown';
	}
}

export function getScreenCaptureAccess(preferred: boolean): ScreenCaptureAccessState {
	return {
		preferred: Boolean(preferred),
		status: readScreenAccessStatus()
	};
}

/**
 * Persists the user's preference and, when enabling, prompts for screen capture
 * access (or opens System Settings). Screen cannot use askForMediaAccess.
 */
export async function requestScreenCaptureAccess(
	preferred: boolean
): Promise<ScreenCaptureAccessState> {
	const wants = Boolean(preferred);
	if (!isDarwin()) {
		return {
			preferred: wants,
			status: 'unsupported',
			message: 'Screen capture permission is managed by the operating system on this platform.'
		};
	}

	let status = readScreenAccessStatus();
	if (!wants) {
		return { preferred: false, status };
	}

	if (status === 'granted') {
		return { preferred: true, status };
	}

	// Trigger the TCC prompt by enumerating desktop sources.
	try {
		await desktopCapturer.getSources({
			types: ['screen'],
			thumbnailSize: { width: 1, height: 1 }
		});
	} catch (error) {
		console.warn('desktopCapturer.getSources failed while requesting screen access:', error);
	}

	status = readScreenAccessStatus();
	if (status === 'granted') {
		return { preferred: true, status };
	}

	const openedSettings = await openScreenCaptureSettings();
	return {
		preferred: true,
		status,
		openedSettings,
		message: openedSettings
			? 'Opened System Settings → Screen & System Audio Recording. Enable Cometline there.'
			: 'Grant Screen & System Audio Recording to Cometline in System Settings.'
	};
}

export async function openScreenCaptureSettings(): Promise<boolean> {
	if (!isDarwin()) return false;
	try {
		await shell.openExternal(MACOS_SCREEN_RECORDING_SETTINGS_URL);
		return true;
	} catch (error) {
		console.error('Failed to open Screen Recording settings:', error);
		return false;
	}
}

export function screenCaptureSupported(): boolean {
	return isDarwin() || process.platform === 'win32';
}

/** Exported for tests. */
export function macosMajorVersion(): number {
	if (!isDarwin()) return 0;
	return Number(os.release().split('.')[0]) || 0;
}
