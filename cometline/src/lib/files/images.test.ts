import { afterEach, describe, expect, it, vi } from 'vitest';
import { copyImageToClipboard } from './images';

class ClipboardItemMock {
	constructor(readonly data: Record<string, Blob>) {}
}

afterEach(() => {
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

describe('copyImageToClipboard', () => {
	it('writes PNG images without converting them', async () => {
		const source = new Blob(['png'], { type: 'image/png' });
		const write = vi.fn().mockResolvedValue(undefined);
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, blob: () => source }));
		vi.stubGlobal('navigator', { clipboard: { write } });
		vi.stubGlobal('ClipboardItem', ClipboardItemMock);
		const createBitmap = vi.fn();
		vi.stubGlobal('createImageBitmap', createBitmap);

		await copyImageToClipboard('image.png', 'image/png');

		const item = write.mock.calls[0][0][0] as ClipboardItemMock;
		expect(item.data).toEqual({ 'image/png': source });
		expect(createBitmap).not.toHaveBeenCalled();
	});

	it('converts JPEG images to PNG before writing them', async () => {
		const source = new Blob(['jpeg'], { type: 'image/jpeg' });
		const converted = new Blob(['png'], { type: 'image/png' });
		const write = vi.fn().mockResolvedValue(undefined);
		const close = vi.fn();
		const drawImage = vi.fn();
		const canvas = {
			width: 0,
			height: 0,
			getContext: vi.fn().mockReturnValue({ drawImage }),
			toBlob: vi.fn().mockImplementation((callback: BlobCallback) => callback(converted))
		};
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, blob: () => source }));
		vi.stubGlobal('navigator', { clipboard: { write } });
		vi.stubGlobal('ClipboardItem', ClipboardItemMock);
		vi.stubGlobal('createImageBitmap', vi.fn().mockResolvedValue({ width: 8, height: 6, close }));
		vi.stubGlobal('document', { createElement: vi.fn().mockReturnValue(canvas) });

		await copyImageToClipboard('image.jpg', 'image/jpeg');

		const item = write.mock.calls[0][0][0] as ClipboardItemMock;
		expect(item.data).toEqual({ 'image/png': converted });
		expect(canvas).toMatchObject({ width: 8, height: 6 });
		expect(drawImage).toHaveBeenCalledOnce();
		expect(close).toHaveBeenCalledOnce();
	});
});
