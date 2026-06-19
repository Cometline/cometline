import type { ImageAttachment } from '$lib/types';

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
			rejected.push({ name: file.name, reason: `Only ${MAX_IMAGE_ATTACHMENTS} images can be attached.` });
			continue;
		}
		if (!isSupportedImageFile(file)) {
			rejected.push({ name: file.name, reason: 'Unsupported image type.' });
			continue;
		}
		if (file.size > MAX_IMAGE_BYTES) {
			rejected.push({ name: file.name, reason: `Image is larger than ${MAX_IMAGE_BYTES / 1024 / 1024} MB.` });
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

export function imageDataURL(image: Pick<ImageAttachment, 'media_type' | 'data'>): string {
	return `data:${image.media_type};base64,${image.data}`;
}
