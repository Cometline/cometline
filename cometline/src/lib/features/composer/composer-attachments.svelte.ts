import { formatDroppedFiles, readDroppedTextFiles } from '$lib/files/dropped-files';
import { isSupportedImageFile, readImageAttachments } from '$lib/files/images';
import type { ImageAttachment } from '$lib/types';
import type { ComposerInputRef } from '$lib/features/composer/composer-input-ref';

export function createComposerAttachmentsController(deps: {
	getValue: () => string;
	getImages: () => ImageAttachment[];
	setImages: (images: ImageAttachment[]) => void;
	getInput: () => ComposerInputRef | null;
}) {
	let dragDepth = $state(0);
	let dropMessage = $state('');
	let dropProcessing = $state(false);
	let dropMessageTimer: ReturnType<typeof setTimeout> | null = null;

	const dragActive = $derived(dragDepth > 0 || dropProcessing);

	function destroy() {
		if (dropMessageTimer) clearTimeout(dropMessageTimer);
	}

	function setDropMessage(message: string) {
		dropMessage = message;
		if (dropMessageTimer) clearTimeout(dropMessageTimer);
		dropMessageTimer = setTimeout(() => {
			dropMessage = '';
			dropMessageTimer = null;
		}, 4200);
	}

	function hasDroppedFiles(dataTransfer: DataTransfer | null): boolean {
		return dataTransfer?.types.includes('Files') ?? false;
	}

	async function addImageFiles(files: File[]) {
		const result = await readImageAttachments(files, deps.getImages().length);
		if (result.accepted.length > 0) {
			deps.setImages([...deps.getImages(), ...result.accepted]);
		}
		if (result.rejected.length > 0) {
			const first = result.rejected[0];
			setDropMessage(`${first.name}: ${first.reason}`);
		} else if (result.accepted.length > 0) {
			setDropMessage(
				`Attached ${result.accepted.length} ${result.accepted.length === 1 ? 'image' : 'images'}.`
			);
		}
	}

	function removeImage(id: string | undefined) {
		deps.setImages(deps.getImages().filter((image) => image.id !== id));
	}

	function onDragEnter(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		dragDepth += 1;
	}

	function onDragOver(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
	}

	function onDragLeave(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		dragDepth = Math.max(0, dragDepth - 1);
	}

	async function onDrop(e: DragEvent) {
		if (!hasDroppedFiles(e.dataTransfer)) return;
		e.preventDefault();
		dragDepth = 0;
		const files = Array.from(e.dataTransfer?.files ?? []);
		if (files.length === 0) return;
		const imageFiles = files.filter(isSupportedImageFile);
		const textFiles = files.filter((file) => !isSupportedImageFile(file));

		dropProcessing = true;
		try {
			if (imageFiles.length > 0) {
				await addImageFiles(imageFiles);
			}
			const result = await readDroppedTextFiles(textFiles);
			if (result.accepted.length > 0) {
				const formatted = formatDroppedFiles(result.accepted);
				const prefix = deps.getValue().trim() ? '\n\n' : '';
				deps.getInput()?.insertText(`${prefix}${formatted}\n`);
			}

			if (textFiles.length === 0) {
				return;
			}
			if (result.accepted.length === 0) {
				const first = result.rejected[0];
				setDropMessage(
					first ? `No files added. ${first.name}: ${first.reason}` : 'No files added.'
				);
			} else if (result.rejected.length > 0) {
				setDropMessage(
					`Added ${result.accepted.length} ${result.accepted.length === 1 ? 'file' : 'files'}. ${result.rejected.length} skipped.`
				);
			} else {
				setDropMessage(
					`Added ${result.accepted.length} ${result.accepted.length === 1 ? 'file' : 'files'}.`
				);
			}
		} catch (err) {
			setDropMessage(err instanceof Error ? err.message : 'Failed to read dropped files.');
		} finally {
			dropProcessing = false;
		}
	}

	return {
		get dragActive() {
			return dragActive;
		},
		get dropMessage() {
			return dropMessage;
		},
		get dropProcessing() {
			return dropProcessing;
		},
		setDropMessage,
		addImageFiles,
		removeImage,
		onDragEnter,
		onDragOver,
		onDragLeave,
		onDrop,
		destroy
	};
}
