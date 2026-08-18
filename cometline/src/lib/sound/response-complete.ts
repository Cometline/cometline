import { browser } from '$app/environment';
import type { ResponseCompleteSoundSettings } from '$lib/types';

const RESPONSE_COMPLETE_SOUND_URL = '/sound/response_complete.mp3';
const ERROR_SOUND_URL = '/sound/error.mp3';

const audioByURL = new Map<string, HTMLAudioElement>();

function clampVolume(volume: number): number {
	if (!Number.isFinite(volume)) return 0;
	return Math.min(1, Math.max(0, volume));
}

function playSound(
	url: string,
	options?: Partial<ResponseCompleteSoundSettings> & { force?: boolean }
) {
	if (!browser) return;

	const enabled = options?.enabled !== false;
	if (!options?.force && !enabled) return;

	const volume = clampVolume(options?.volume ?? 1);
	if (volume <= 0) return;

	try {
		let audio = audioByURL.get(url);
		if (!audio) {
			audio = new Audio(url);
			audioByURL.set(url, audio);
		}
		audio.volume = volume;
		audio.currentTime = 0;
		void audio.play().catch(() => {});
	} catch {
		// Ignore playback failures (autoplay policy, missing file, etc.).
	}
}

/** Play the successful or cancelled agent-run chime. */
export function playResponseCompleteSound(
	options?: Partial<ResponseCompleteSoundSettings> & { force?: boolean }
) {
	playSound(RESPONSE_COMPLETE_SOUND_URL, options);
}

/** Play the failed agent-run chime. */
export function playErrorSound(options?: Partial<ResponseCompleteSoundSettings>) {
	playSound(ERROR_SOUND_URL, options);
}
