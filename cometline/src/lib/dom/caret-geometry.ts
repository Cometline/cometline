const TEXT_FIELD_MIRROR_STYLES = [
	'fontStyle',
	'fontVariant',
	'fontWeight',
	'fontStretch',
	'fontSize',
	'fontFamily',
	'letterSpacing',
	'wordSpacing',
	'textTransform',
	'textIndent',
	'textAlign',
	'textDecoration',
	'paddingTop',
	'paddingRight',
	'paddingBottom',
	'paddingLeft',
	'borderTopWidth',
	'borderRightWidth',
	'borderBottomWidth',
	'borderLeftWidth',
	'boxSizing',
	'lineHeight'
] as const;

/** Native inputs/textareas have no Selection range; measure via a styled mirror. */
export function readTextFieldCaretClientRect(
	field: HTMLInputElement | HTMLTextAreaElement
): DOMRect | null {
	const style = getComputedStyle(field);
	const fieldRect = field.getBoundingClientRect();
	if (fieldRect.width === 0 && fieldRect.height === 0) return null;

	const isMultiline = field instanceof HTMLTextAreaElement;
	const mirror = document.createElement('div');
	mirror.setAttribute('aria-hidden', 'true');
	mirror.style.position = 'fixed';
	mirror.style.left = `${fieldRect.left}px`;
	mirror.style.top = `${fieldRect.top}px`;
	mirror.style.width = `${fieldRect.width}px`;
	mirror.style.height = `${fieldRect.height}px`;
	mirror.style.visibility = 'hidden';
	mirror.style.pointerEvents = 'none';
	mirror.style.overflow = 'hidden';
	mirror.style.whiteSpace = isMultiline ? 'pre-wrap' : 'pre';
	mirror.style.overflowWrap = isMultiline ? 'break-word' : 'normal';
	for (const prop of TEXT_FIELD_MIRROR_STYLES) {
		mirror.style[prop] = style[prop];
	}

	const pos = field.selectionEnd ?? field.value.length;
	mirror.append(field.value.slice(0, pos));
	const marker = document.createElement('span');
	marker.textContent = '\u200b';
	mirror.append(marker);
	if (isMultiline) {
		mirror.append(field.value.slice(pos));
	}

	document.body.append(mirror);
	mirror.scrollLeft = field.scrollLeft;
	mirror.scrollTop = field.scrollTop;
	const rect = marker.getBoundingClientRect();
	mirror.remove();
	return rect;
}

export function textFieldCaretLineHeight(field: HTMLInputElement | HTMLTextAreaElement): number {
	const parsed = Number.parseFloat(getComputedStyle(field).lineHeight);
	if (Number.isFinite(parsed) && parsed > 0) return parsed;
	return field.clientHeight || 22.5;
}

export function readTextFieldCaretLocal(
	wrap: HTMLElement,
	field: HTMLInputElement | HTMLTextAreaElement
): { x: number; y: number; h: number } | null {
	const rect = readTextFieldCaretClientRect(field);
	if (!rect) return null;
	return viewportDeltaToLocal(wrap, rect, textFieldCaretLineHeight(field));
}

/** Convert a viewport-space caret rect to wrap-local coordinates. */
export function viewportDeltaToLocal(
	wrap: HTMLElement,
	rect: Pick<DOMRect, 'left' | 'top' | 'height'>,
	lineHeight: number
): { x: number; y: number; h: number } {
	const wrapRect = wrap.getBoundingClientRect();
	const scaleX = wrapRect.width > 0 ? wrap.offsetWidth / wrapRect.width : 1;
	const scaleY = wrapRect.height > 0 ? wrap.offsetHeight / wrapRect.height : 1;

	return {
		x: (rect.left - wrapRect.left) * scaleX,
		y: (rect.top - wrapRect.top) * scaleY,
		h: (rect.height || lineHeight) * scaleY
	};
}
