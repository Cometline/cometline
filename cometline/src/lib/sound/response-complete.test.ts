// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const play = vi.fn(() => Promise.resolve());
const audioInstances: Array<{
	src?: string;
	volume: number;
	currentTime: number;
	play: typeof play;
}> = [];

class MockAudio {
	src?: string;
	volume = 1;
	currentTime = 0;
	play = play;

	constructor(src?: string) {
		this.src = src;
		audioInstances.push(this);
	}
}

describe('agent run sounds', () => {
	beforeEach(() => {
		audioInstances.length = 0;
		play.mockClear();
		vi.stubGlobal('Audio', MockAudio);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.resetModules();
	});

	it('plays at the given volume when enabled', async () => {
		const { playResponseCompleteSound } = await import('./response-complete');
		playResponseCompleteSound({ enabled: true, volume: 0.4 });

		expect(audioInstances).toHaveLength(1);
		expect(audioInstances[0]?.src).toBe('/sound/response_complete.mp3');
		expect(audioInstances[0]?.volume).toBe(0.4);
		expect(audioInstances[0]?.currentTime).toBe(0);
		expect(play).toHaveBeenCalledTimes(1);
	});

	it('plays the error sound at the same configured volume', async () => {
		const { playErrorSound } = await import('./response-complete');
		playErrorSound({ enabled: true, volume: 0.4 });

		expect(audioInstances).toHaveLength(1);
		expect(audioInstances[0]?.src).toBe('/sound/error.mp3');
		expect(audioInstances[0]?.volume).toBe(0.4);
		expect(audioInstances[0]?.currentTime).toBe(0);
		expect(play).toHaveBeenCalledTimes(1);
	});

	it('no-ops when disabled', async () => {
		const { playResponseCompleteSound } = await import('./response-complete');
		playResponseCompleteSound({ enabled: false, volume: 0.9 });

		expect(audioInstances).toHaveLength(0);
		expect(play).not.toHaveBeenCalled();
	});

	it('no-ops when volume is 0', async () => {
		const { playResponseCompleteSound } = await import('./response-complete');
		playResponseCompleteSound({ enabled: true, volume: 0 });

		expect(audioInstances).toHaveLength(0);
		expect(play).not.toHaveBeenCalled();
	});

	it('ignores enabled when force is true', async () => {
		const { playResponseCompleteSound } = await import('./response-complete');
		playResponseCompleteSound({ enabled: false, volume: 0.55, force: true });

		expect(audioInstances).toHaveLength(1);
		expect(audioInstances[0]?.volume).toBe(0.55);
		expect(play).toHaveBeenCalledTimes(1);
	});
});
