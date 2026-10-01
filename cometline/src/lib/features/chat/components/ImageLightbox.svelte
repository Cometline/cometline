<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import { X } from '@lucide/svelte';

	const MIN_ZOOM = 0.5;
	const MAX_ZOOM = 8;
	const CAPTION_MAX_LEN = 80;

	let {
		open = false,
		src,
		alt = 'Image preview',
		onClose
	}: {
		open?: boolean;
		src: string;
		alt?: string;
		onClose: () => void;
	} = $props();

	/** Zoom model: transform-origin center + translate(offset) scale(s). */
	let scaleFactor = $state(1);
	let offsetX = $state(0);
	let offsetY = $state(0);
	let dragging = $state(false);
	let surfaceEl: HTMLElement | undefined;
	let dragStartX = 0;
	let dragStartY = 0;
	let dragOriginX = 0;
	let dragOriginY = 0;

	const canPan = $derived(scaleFactor > 1);
	const surfaceCursor = $derived(canPan ? (dragging ? 'grabbing' : 'grab') : 'zoom-in');
	const surfaceStyle = $derived(
		`transform: translate(${offsetX}px, ${offsetY}px) scale(${scaleFactor}); cursor: ${surfaceCursor};`
	);
	const showCaption = $derived(
		Boolean(alt && alt !== 'Image preview' && alt.length <= CAPTION_MAX_LEN)
	);

	function resetTransform() {
		scaleFactor = 1;
		offsetX = 0;
		offsetY = 0;
		dragging = false;
	}

	function bindSurface(el: HTMLElement) {
		surfaceEl = el;
		return () => {
			if (surfaceEl === el) surfaceEl = undefined;
		};
	}

	function focusClose(button: HTMLButtonElement) {
		queueMicrotask(() => button.focus());
	}

	function openModal(dialog: HTMLDialogElement) {
		dialog.showModal();
		resetTransform();
		return () => {
			dialog.close();
			resetTransform();
		};
	}

	function cancel(event: Event) {
		event.preventDefault();
		onClose();
	}

	function onKeydown(event: KeyboardEvent) {
		if (!open) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopImmediatePropagation();
			onClose();
		}
	}

	function clampScale(value: number) {
		return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, value));
	}

	/**
	 * Cursor-anchored zoom around transform-origin: center.
	 * Keeps the screen point under (clientX, clientY) fixed while scale changes
	 * by adjusting translate: offset += deltaFromCenter * (1 - next/prev).
	 */
	function zoomAt(clientX: number, clientY: number, nextScale: number) {
		const prev = scaleFactor;
		const next = clampScale(nextScale);
		if (next === prev) return;

		const el = surfaceEl;
		if (el) {
			const rect = el.getBoundingClientRect();
			const cx = rect.left + rect.width / 2;
			const cy = rect.top + rect.height / 2;
			const dx = clientX - cx;
			const dy = clientY - cy;
			const ratio = next / prev;
			offsetX += dx * (1 - ratio);
			offsetY += dy * (1 - ratio);
		}

		scaleFactor = next;
		if (next <= 1) {
			offsetX = 0;
			offsetY = 0;
		}
	}

	/**
	 * macOS Photos-like gestures:
	 * - Pinch (wheel + ctrlKey) → continuous exponential zoom toward pointer
	 * - Two-finger scroll → pan when zoomed; never zooms
	 * Mouse wheel without ctrl also pans when zoomed (does not step-zoom).
	 */
	function onWheel(event: WheelEvent) {
		const isPinch = event.ctrlKey || event.metaKey;

		if (isPinch) {
			event.preventDefault();
			// Pixel deltas from trackpad pinch; exponential keeps speed perceptually even
			// across scale levels (closer to Photos / Maps than fixed 1.1 steps).
			let dy = event.deltaY;
			if (event.deltaMode === 1) dy *= 16; // DOM_DELTA_LINE
			if (event.deltaMode === 2) dy *= 32; // DOM_DELTA_PAGE
			const next = scaleFactor * Math.exp(-dy * 0.011);
			zoomAt(event.clientX, event.clientY, next);
			return;
		}

		if (scaleFactor > 1) {
			event.preventDefault();
			let dx = event.deltaX;
			let dy = event.deltaY;
			if (event.deltaMode === 1) {
				dx *= 16;
				dy *= 16;
			} else if (event.deltaMode === 2) {
				dx *= 32;
				dy *= 32;
			}
			offsetX -= dx;
			offsetY -= dy;
		}
		// At scale 1, ignore two-finger scroll (don't zoom, don't steal the gesture).
	}

	function onPointerDown(event: PointerEvent) {
		if (!canPan) return;
		event.preventDefault();
		dragging = true;
		dragStartX = event.clientX;
		dragStartY = event.clientY;
		dragOriginX = offsetX;
		dragOriginY = offsetY;
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
	}

	function onPointerMove(event: PointerEvent) {
		if (!dragging) return;
		offsetX = dragOriginX + (event.clientX - dragStartX);
		offsetY = dragOriginY + (event.clientY - dragStartY);
	}

	function onPointerUp(event: PointerEvent) {
		if (!dragging) return;
		dragging = false;
		try {
			(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId);
		} catch {
			/* already released */
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<dialog
		{@attach openModal}
		class="image-lightbox"
		aria-label="Image preview"
		oncancel={cancel}
		onwheel={onWheel}
		transition:fade={{ duration: 120 }}
	>
		<button type="button" class="scrim" aria-label="Close image preview" onclick={onClose}
		></button>
		<button
			{@attach focusClose}
			type="button"
			class="close-btn"
			aria-label="Close image preview"
			onclick={onClose}
		>
			<X size={16} />
		</button>
		<div class="stage" transition:scale|global={{ start: 0.97, duration: 140 }}>
			<div
				{@attach bindSurface}
				class="surface"
				class:has-caption={showCaption}
				role="presentation"
				style={surfaceStyle}
				onpointerdown={onPointerDown}
				onpointermove={onPointerMove}
				onpointerup={onPointerUp}
				onpointercancel={onPointerUp}
			>
				<img class="preview" {src} {alt} draggable="false" />
				{#if showCaption}
					<p class="caption">{alt}</p>
				{/if}
			</div>
		</div>
	</dialog>
{/if}

<style>
	.image-lightbox {
		position: fixed;
		inset: 0;
		width: 100vw;
		max-width: none;
		height: 100vh;
		max-height: none;
		margin: 0;
		padding: 24px;
		border: none;
		background: transparent;
		overflow: hidden;
		display: grid;
		place-items: center;
	}

	.image-lightbox::backdrop {
		background: rgba(17, 24, 39, 0.18);
		backdrop-filter: blur(12px);
	}

	.scrim {
		position: absolute;
		inset: 0;
		z-index: 0;
		margin: 0;
		padding: 0;
		border: none;
		background: transparent;
		cursor: default;
	}

	.close-btn {
		position: fixed;
		top: 16px;
		right: 16px;
		z-index: 2;
		margin: 0;
		padding: 4px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
		display: grid;
		place-items: center;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			color var(--duration-fast) var(--ease-smooth);
	}

	.close-btn:hover {
		background: rgba(15, 23, 42, 0.05);
		color: var(--text-main);
	}

	.close-btn:focus-visible {
		outline: 2px solid color-mix(in srgb, var(--accent) 70%, white);
		outline-offset: 2px;
	}

	.stage {
		position: relative;
		z-index: 1;
		max-width: min(92vw, 1200px);
		max-height: min(88vh, 100%);
	}

	/* Single transform unit: panel chrome + image scale/pan together. */
	.surface {
		max-width: min(92vw, 1200px);
		max-height: min(88vh, 100%);
		padding: 12px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card, 16px);
		box-shadow: var(--shadow-card);
		transform-origin: center center;
		touch-action: none;
		user-select: none;
		will-change: transform;
	}

	.preview {
		display: block;
		max-width: min(calc(92vw - 48px), 1176px);
		max-height: calc(88vh - 24px);
		object-fit: contain;
		border-radius: calc(var(--radius-card, 16px) - 8px);
		pointer-events: none;
		-webkit-user-drag: none;
	}

	.surface.has-caption .preview {
		max-height: calc(88vh - 52px);
	}

	.caption {
		margin: 0;
		max-width: 100%;
		padding: 0 4px 2px;
		font-size: 12px;
		line-height: 1.4;
		color: var(--text-muted);
		text-align: center;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		pointer-events: none;
	}
</style>
