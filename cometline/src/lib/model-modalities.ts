export type InputModality = 'text' | 'image' | 'video' | 'audio' | 'pdf';

export const INPUT_MODALITY_ORDER: InputModality[] = ['text', 'image', 'video', 'audio', 'pdf'];

export const INPUT_MODALITY_LABEL: Record<InputModality, string> = {
	text: 'TEXT',
	image: 'IMAGE',
	video: 'VIDEO',
	audio: 'AUDIO',
	pdf: 'DOCUMENT'
};
