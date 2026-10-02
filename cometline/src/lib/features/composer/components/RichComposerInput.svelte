<script lang="ts">
	import { fly } from 'svelte/transition';
	import type { CaretTrailSettings } from '#lib/types.js';
	import { customCaret } from '#lib/dom/custom-caret.js';
	import { createRichComposerInputController } from '#lib/features/composer/rich-composer-input.svelte.js';

	let {
		value = $bindable(''),
		placeholder = '',
		ariaLabel = 'Message input',
		skillNames = [],
		caretTrail = { enabled: true, intensity: 0.72, speed: 0.68 },
		caretColor = 'var(--color-72c0ff)',
		mentionsEnabled = true,
		onkeydown,
		onfiles,
		onmentionquery
	}: {
		value?: string;
		placeholder?: string;
		ariaLabel?: string;
		skillNames?: string[];
		caretTrail?: CaretTrailSettings;
		caretColor?: string;
		mentionsEnabled?: boolean;
		onkeydown?: (e: KeyboardEvent) => void;
		onfiles?: (files: File[]) => void;
		onmentionquery?: (payload: { query: string; active: boolean }) => void;
	} = $props();

	let wrap = $state<HTMLDivElement | null>(null);
	let editor = $state<HTMLDivElement | null>(null);
	let caretEl = $state<HTMLSpanElement | null>(null);
	let trailPoly = $state<SVGPolygonElement | null>(null);

	let caretTrailEnabled = $derived(caretTrail.enabled);

	const rich = createRichComposerInputController({
		getEditor: () => editor,
		setValue: (next) => {
			value = next;
		},
		getSkillNames: () => skillNames,
		getMentionsEnabled: () => mentionsEnabled,
		onkeydown: (e) => onkeydown?.(e),
		onfiles: (files) => onfiles?.(files),
		onmentionquery: (payload) => onmentionquery?.(payload)
	});

	export function focus(options?: { position?: 'start' | 'end' }) {
		rich.focus(options);
	}

	export function focusAsync(options?: { position?: 'start' | 'end' }): Promise<void> {
		return rich.focusAsync(options);
	}

	export function insertText(text: string) {
		rich.insertText(text);
	}

	export function insertFileMention(path: string) {
		rich.insertFileMention(path);
	}

	export function getFilePaths(): string[] {
		return rich.getFilePaths();
	}

	export function setText(text: string) {
		rich.setText(text);
	}

	/** Clears the editor (used after send). */
	export function clear() {
		rich.clear();
	}

	export function isCaretAtStart(): boolean {
		return rich.isCaretAtStart();
	}

	export function isCaretAtEnd(): boolean {
		return rich.isCaretAtEnd();
	}

	// Keep the DOM in sync when `value` is set externally to empty (e.g. cleared
	// after send). We only handle the clear case to avoid clobbering chips.
	$effect(() => {
		if (value === '' && editor && editor.textContent !== '') {
			// eslint-disable-next-line svelte/no-dom-manipulating -- contenteditable children are owned by the editor, not Svelte
			editor.innerHTML = '';
		}
	});

	$effect(() => {
		const key = skillNames.join('\n');
		if (!editor || key === '') return;
		rich.syncDecorations();
	});

	$effect(() => {
		const onSelectionChange = () => rich.updateMentionState();
		document.addEventListener('selectionchange', onSelectionChange);
		return () => {
			document.removeEventListener('selectionchange', onSelectionChange);
		};
	});

	// Placeholder hides as soon as the user has typed anything, including spaces.
	// Phantom empty-editor <br> is normalized to '' in readValue.
	let isEmpty = $derived(value === '');
</script>

<div bind:this={wrap} class="rce-wrap">
	{#if isEmpty}
		{#key placeholder}
			<div class="rce-placeholder" in:fly={{ y: 10, duration: 1000 }} aria-hidden="true">
				{placeholder}
			</div>
		{/key}
	{/if}
	{#if caretTrailEnabled}
		<div
			class="rce-caret-layer"
			class:visible={rich.focused && rich.caretReady}
			aria-hidden="true"
		>
			<svg class="rce-trail" focusable="false">
				<polygon bind:this={trailPoly}></polygon>
			</svg>
			<span bind:this={caretEl} class="rce-caret"></span>
		</div>
	{/if}
	<div
		bind:this={editor}
		class="rce-editor scrollbar-none"
		class:trail-enabled={caretTrailEnabled}
		use:customCaret={{
			wrap,
			caret: caretEl,
			trail: trailPoly,
			caretTrail,
			color: caretColor,
			onStateChange: rich.onCaretStateChange
		}}
		contenteditable="true"
		role="textbox"
		tabindex="0"
		aria-multiline="true"
		aria-label={ariaLabel}
		oninput={rich.onInput}
		onpaste={rich.onPaste}
		onkeydown={rich.onKeydownInternal}
		oncompositionstart={rich.onCompositionStart}
		oncompositionend={rich.onCompositionEnd}
		onclick={rich.onEditorClick}
		onfocus={rich.updateMentionState}
		onblur={rich.updateMentionState}
	></div>
</div>

<style>
	.rce-wrap {
		position: relative;
		width: 100%;
	}

	.rce-placeholder {
		position: absolute;
		inset: 0;
		pointer-events: none;
		color: var(--text-soft);
		font-size: 15px;
		line-height: 1.5;
		white-space: pre-wrap;
	}

	.rce-editor {
		width: 100%;
		min-height: calc(1.5em * 3);
		max-height: calc(1.5em * 8);
		overflow-y: auto;
		font-size: 15px;
		line-height: 1.5;
		color: var(--text-main);
		outline: none;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: inherit;
	}

	.rce-editor.trail-enabled {
		caret-color: transparent;
	}

	.rce-caret-layer {
		position: absolute;
		inset: 0;
		pointer-events: none;
		z-index: 2;
		overflow: hidden;
		opacity: 0;
		transition: opacity 0.08s ease;
	}

	.rce-caret-layer.visible {
		opacity: 1;
	}

	.rce-trail {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		overflow: visible;
	}

	.rce-trail polygon {
		fill: var(--rce-caret-color);
		stroke: none;
		opacity: 0;
		filter: drop-shadow(0 0 6px var(--rce-caret-color));
	}

	@keyframes rce-caret-blink {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.75;
		}
	}

	.rce-caret {
		position: absolute;
		top: 0;
		left: 0;
		width: 2px;
		height: 1.5em;
		border-radius: 999px;
		background: var(--rce-caret-color);
		box-shadow: 0 0 9px var(--rce-caret-color);
		will-change: transform;
		animation: rce-caret-blink 1.1s ease-in-out infinite;
	}

	:global(.rce-caret.moving) {
		animation: none;
	}

	.rce-caret::after {
		content: '';
		position: absolute;
		inset: -5px -4px;
		border-radius: 999px;
		background: var(--rce-caret-color);
		opacity: 0.14;
		filter: blur(5px);
	}

	.rce-editor :global(.rce-chip) {
		display: inline-flex;
		align-items: center;
		gap: 0.3em;
		max-width: 16rem;
		vertical-align: middle;
		padding: 0.1em 0.45em;
		margin: 0 2px;
		border: 1px solid var(--border-soft);
		border-radius: 6px;
		background: rgba(15, 23, 42, 0.04);
		font-size: 0.92em;
		line-height: 1.3;
		color: var(--text-muted);
		white-space: nowrap;
		user-select: none;
		cursor: pointer;
	}

	.rce-editor :global(.rce-skill-chip) {
		border-color: rgba(37, 99, 235, 0.18);
		background: rgba(37, 99, 235, 0.06);
		color: var(--color-31517a);
		font-weight: 650;
	}

	.rce-editor :global(.rce-file-chip) {
		border-color: rgba(16, 185, 129, 0.22);
		background: rgba(16, 185, 129, 0.07);
		color: var(--color-1d5c42);
		font-weight: 650;
	}

	.rce-editor :global(.rce-dir-chip) {
		border-color: rgba(59, 130, 246, 0.22);
		background: rgba(59, 130, 246, 0.07);
		color: var(--color-1e3a5f);
		font-weight: 650;
	}

	.rce-editor :global(.rce-chip-icon) {
		flex-shrink: 0;
		width: 1em;
		height: 1em;
		object-fit: contain;
		border-radius: 3px;
	}

	.rce-editor :global(.rce-chip-label) {
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
