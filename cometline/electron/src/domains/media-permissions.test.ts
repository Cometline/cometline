import { describe, expect, it, vi } from 'vitest';

vi.mock('electron', () => ({
	desktopCapturer: {
		getSources: vi.fn(async () => [])
	},
	shell: {
		openExternal: vi.fn(async () => undefined)
	},
	systemPreferences: {
		getMediaAccessStatus: vi.fn(() => 'not-determined')
	}
}));

import { desktopCapturer, systemPreferences } from 'electron';
import {
	getScreenCaptureAccess,
	requestScreenCaptureAccess
} from './media-permissions';

describe('media-permissions', () => {
	it('reports preferred flag with OS status', () => {
		vi.mocked(systemPreferences.getMediaAccessStatus).mockReturnValue('granted');
		expect(getScreenCaptureAccess(true)).toEqual({
			preferred: true,
			status: 'granted'
		});
	});

	it('requests capture by enumerating desktop sources when enabling', async () => {
		vi.mocked(systemPreferences.getMediaAccessStatus)
			.mockReturnValueOnce('not-determined')
			.mockReturnValueOnce('granted');
		const result = await requestScreenCaptureAccess(true);
		expect(desktopCapturer.getSources).toHaveBeenCalled();
		expect(result).toMatchObject({ preferred: true, status: 'granted' });
	});
});
