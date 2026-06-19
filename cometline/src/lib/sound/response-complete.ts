import { browser } from '$app/environment';

const RESPONSE_COMPLETE_SOUND_URL = '/sound/response_complete.mp3';

let audio: HTMLAudioElement | null = null;

export function playResponseCompleteSound() {
	if (!browser) return;

	try {
		audio ??= new Audio(RESPONSE_COMPLETE_SOUND_URL);
		audio.currentTime = 0;
		void audio.play().catch(() => {});
	} catch {
		// Ignore playback failures (autoplay policy, missing file, etc.).
	}
}
