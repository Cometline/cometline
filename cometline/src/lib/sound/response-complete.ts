import { browser } from '$app/environment';
import type { ResponseCompleteSoundSettings } from '$lib/types';

const RESPONSE_COMPLETE_SOUND_URL = '/sound/response_complete.mp3';

let audio: HTMLAudioElement | null = null;

function clampVolume(volume: number): number {
	if (!Number.isFinite(volume)) return 0;
	return Math.min(1, Math.max(0, volume));
}

/**
 * Play the response-complete chime.
 * @param options.enabled when false, no-op. Defaults to true when omitted.
 * @param options.volume 0–1 gain (HTMLAudioElement.volume). Defaults to 1 when omitted.
 * @param options.force when true, ignore `enabled` (used by settings preview).
 */
export function playResponseCompleteSound(
	options?: Partial<ResponseCompleteSoundSettings> & { force?: boolean }
) {
	if (!browser) return;

	const enabled = options?.enabled !== false;
	if (!options?.force && !enabled) return;

	const volume = clampVolume(options?.volume ?? 1);
	if (volume <= 0) return;

	try {
		audio ??= new Audio(RESPONSE_COMPLETE_SOUND_URL);
		audio.volume = volume;
		audio.currentTime = 0;
		void audio.play().catch(() => {});
	} catch {
		// Ignore playback failures (autoplay policy, missing file, etc.).
	}
}
