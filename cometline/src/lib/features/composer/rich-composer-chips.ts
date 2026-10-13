import { faviconUrl, domainFromUrl, fileMentionText } from '#lib/markdown/embed.js';
import { pastedTextLabel } from '#lib/features/composer/rich-composer-paste.js';

/** Build a non-editable inline chip element for a URL. */
export function makeChip(url: string): HTMLElement {
	const chip = document.createElement('span');
	chip.className = 'rce-chip';
	chip.contentEditable = 'false';
	chip.dataset.url = url;
	chip.title = url;

	const img = document.createElement('img');
	img.className = 'rce-chip-icon';
	img.src = faviconUrl(url);
	img.alt = '';
	img.width = 14;
	img.height = 14;
	img.addEventListener('error', () => (img.style.visibility = 'hidden'));

	const label = document.createElement('span');
	label.className = 'rce-chip-label';
	label.textContent = domainFromUrl(url);

	chip.appendChild(img);
	chip.appendChild(label);
	return chip;
}

export function makeSkillChip(name: string): HTMLElement {
	const chip = document.createElement('span');
	chip.className = 'rce-chip rce-skill-chip';
	chip.contentEditable = 'false';
	chip.dataset.skillCommand = `/${name}`;
	chip.title = `Use the ${name} skill`;

	const label = document.createElement('span');
	label.className = 'rce-chip-label';
	label.textContent = `/${name}`;
	chip.appendChild(label);
	return chip;
}

export function makeFileChip(path: string): HTMLElement {
	const chip = document.createElement('span');
	chip.className = 'rce-chip rce-file-chip';
	chip.contentEditable = 'false';
	chip.dataset.filePath = path;
	chip.title = path;

	const label = document.createElement('span');
	label.className = 'rce-chip-label';
	label.textContent = fileMentionText(path);
	chip.appendChild(label);
	return chip;
}

export function makeDirChip(path: string): HTMLElement {
	const normalized = path.endsWith('/') ? path : `${path}/`;
	const chip = document.createElement('span');
	chip.className = 'rce-chip rce-dir-chip';
	chip.contentEditable = 'false';
	chip.dataset.filePath = normalized;
	chip.title = normalized;

	const label = document.createElement('span');
	label.className = 'rce-chip-label';
	label.textContent = fileMentionText(normalized);
	chip.appendChild(label);
	return chip;
}

/** Non-editable stand-in for a large paste. The full text lives on the dataset, not in the layout. */
export function makePasteChip(text: string): HTMLElement {
	const chip = document.createElement('span');
	chip.className = 'rce-chip rce-paste-chip';
	chip.contentEditable = 'false';
	const labelText = pastedTextLabel(text);
	chip.dataset.pastedText = text;
	chip.title = labelText;
	chip.setAttribute('role', 'img');
	chip.setAttribute('aria-label', labelText);

	const label = document.createElement('span');
	label.className = 'rce-chip-label';
	label.textContent = labelText;
	chip.appendChild(label);
	return chip;
}
