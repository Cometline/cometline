import type { ImageAttachment, MediaAttachment } from '$lib/types';

export const MAX_IMAGE_ATTACHMENTS = 6;
export const MAX_IMAGE_BYTES = 4 * 1024 * 1024;

const SUPPORTED_IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp']);

export interface ImageReadResult {
	accepted: ImageAttachment[];
	rejected: { name: string; reason: string }[];
}

function readAsDataURL(file: File): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
		reader.onload = () => resolve(String(reader.result ?? ''));
		reader.readAsDataURL(file);
	});
}

export function isSupportedImageFile(file: File): boolean {
	return SUPPORTED_IMAGE_TYPES.has(file.type.toLowerCase());
}

export async function readImageAttachments(
	files: File[],
	existingCount = 0
): Promise<ImageReadResult> {
	const accepted: ImageAttachment[] = [];
	const rejected: { name: string; reason: string }[] = [];

	for (const file of files) {
		if (existingCount + accepted.length >= MAX_IMAGE_ATTACHMENTS) {
			rejected.push({
				name: file.name,
				reason: `Only ${MAX_IMAGE_ATTACHMENTS} images can be attached.`
			});
			continue;
		}
		if (!isSupportedImageFile(file)) {
			rejected.push({ name: file.name, reason: 'Unsupported image type.' });
			continue;
		}
		if (file.size > MAX_IMAGE_BYTES) {
			rejected.push({
				name: file.name,
				reason: `Image is larger than ${MAX_IMAGE_BYTES / 1024 / 1024} MB.`
			});
			continue;
		}

		const dataURL = await readAsDataURL(file);
		const comma = dataURL.indexOf(',');
		if (comma < 0) {
			rejected.push({ name: file.name, reason: 'Could not read image data.' });
			continue;
		}

		accepted.push({
			id: crypto.randomUUID(),
			media_type: file.type.toLowerCase() as ImageAttachment['media_type'],
			data: dataURL.slice(comma + 1),
			name: file.name,
			size: file.size
		});
	}

	return { accepted, rejected };
}

export function imageDataURL(
	image: Pick<MediaAttachment, 'media_type' | 'data'> & { data_url?: string }
): string {
	if (image.data_url) return image.data_url;
	if (image.data) return `data:${image.media_type};base64,${image.data}`;
	return '';
}

/** URL for assistant media-store files loaded by id (no inline base64). */
export function sessionMediaURL(sessionId: string, mediaId: string): string {
	return `http://127.0.0.1:7700/api/v1/sessions/${encodeURIComponent(sessionId)}/media/${encodeURIComponent(mediaId)}`;
}

/** URL for cataloged gallery media that still works after the session is deleted. */
export function mediaContentURL(mediaId: string): string {
	return `http://127.0.0.1:7700/api/v1/media/${encodeURIComponent(mediaId)}/content`;
}

async function imageBlobAsPng(source: Blob): Promise<Blob> {
	if (source.type === 'image/png') return source;

	const bitmap = await createImageBitmap(source);
	try {
		const canvas = document.createElement('canvas');
		canvas.width = bitmap.width;
		canvas.height = bitmap.height;
		const context = canvas.getContext('2d');
		if (!context) throw new Error('Could not prepare this image for copying.');
		context.drawImage(bitmap, 0, 0);

		return await new Promise<Blob>((resolve, reject) => {
			canvas.toBlob((blob) => {
				if (blob) resolve(blob);
				else reject(new Error('Could not prepare this image for copying.'));
			}, 'image/png');
		});
	} finally {
		bitmap.close();
	}
}

export async function copyImageToClipboard(src: string, mediaType = 'image/png'): Promise<void> {
	if (!navigator.clipboard?.write || typeof ClipboardItem === 'undefined') {
		throw new Error('Clipboard copy is not available here. Use Download instead.');
	}
	if (mediaType.startsWith('video/')) {
		throw new Error('Video cannot be copied to the clipboard. Use Download instead.');
	}
	const response = await fetch(src);
	if (!response.ok) {
		throw new Error('Could not copy this file.');
	}
	const source = await response.blob();
	const type = mediaType || source.type || 'image/png';
	const blob = type === source.type ? source : new Blob([await source.arrayBuffer()], { type });
	const clipboardBlob = await imageBlobAsPng(blob);
	await navigator.clipboard.write([new ClipboardItem({ 'image/png': clipboardBlob })]);
}

export async function copyMediaFileToClipboard(sessionId: string, mediaId: string): Promise<void> {
	const api = typeof window !== 'undefined' ? window.electronAPI : undefined;
	if (!api?.copyMediaFile) {
		throw new Error('Video file copy is not available here. Use Download instead.');
	}
	const result = await api.copyMediaFile(sessionId, mediaId);
	if (!result.ok) throw new Error(result.error);
}

export function resolveImageSrc(
	image: MediaAttachment & { data_url?: string; media_type?: string },
	sessionId?: string | null
): string {
	const inline = imageDataURL(image);
	if (inline) return inline;
	if (image.id && sessionId) return sessionMediaURL(sessionId, image.id);
	return '';
}

export function isVideoAttachment(
	image: Pick<MediaAttachment, 'media_type'> | { media_type?: string }
): image is MediaAttachment & { media_type: 'video/mp4' | 'video/webm' } {
	return String(image.media_type ?? '').startsWith('video/');
}
