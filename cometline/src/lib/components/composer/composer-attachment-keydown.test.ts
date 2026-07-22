import { describe, expect, it } from 'vitest';
import { nextAttachmentRemoval } from './composer-attachment-keydown';

describe('nextAttachmentRemoval', () => {
	it('returns null when text is non-empty', () => {
		expect(
			nextAttachmentRemoval('hello', [{ id: 'img-1' }], 2)
		).toBeNull();
		expect(
			nextAttachmentRemoval('  x  ', [{ id: 'img-1' }], 2)
		).toBeNull();
	});

	it('treats empty string and whitespace-only as empty', () => {
		expect(nextAttachmentRemoval('', [{ id: 'img-1' }], 0)).toEqual({
			kind: 'image',
			id: 'img-1'
		});
		expect(nextAttachmentRemoval('   \n\t  ', [{ id: 'img-1' }], 0)).toEqual({
			kind: 'image',
			id: 'img-1'
		});
	});

	it('prefers the last image with an id', () => {
		expect(
			nextAttachmentRemoval('', [{ id: 'a' }, { id: 'b' }, { id: 'c' }], 3)
		).toEqual({ kind: 'image', id: 'c' });
	});

	it('skips images without ids and falls through to web context', () => {
		expect(nextAttachmentRemoval('', [{}, {}], 2)).toEqual({
			kind: 'webContext',
			index: 1
		});
	});

	it('removes the last web context when there are no images', () => {
		expect(nextAttachmentRemoval('', [], 3)).toEqual({
			kind: 'webContext',
			index: 2
		});
	});

	it('returns null when empty and nothing is attached', () => {
		expect(nextAttachmentRemoval('', [], 0)).toBeNull();
		expect(nextAttachmentRemoval('  ', [{}], 0)).toBeNull();
	});
});
