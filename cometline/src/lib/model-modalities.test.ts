import { describe, expect, it } from 'vitest';
import { INPUT_MODALITY_LABEL, INPUT_MODALITY_ORDER } from '$lib/model-modalities';

describe('model-modalities', () => {
	it('orders icons text → image → video → audio → pdf', () => {
		expect(INPUT_MODALITY_ORDER).toEqual(['text', 'image', 'video', 'audio', 'pdf']);
	});

	it('labels match OpenCode-style tooltips', () => {
		expect(INPUT_MODALITY_LABEL.text).toBe('TEXT');
		expect(INPUT_MODALITY_LABEL.image).toBe('IMAGE');
		expect(INPUT_MODALITY_LABEL.video).toBe('VIDEO');
		expect(INPUT_MODALITY_LABEL.audio).toBe('AUDIO');
		expect(INPUT_MODALITY_LABEL.pdf).toBe('DOCUMENT');
	});
});
