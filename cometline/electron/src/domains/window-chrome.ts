import type { ShellWindowContext } from './runtime-context.js';

interface SidebarState {
	open?: unknown;
	duration?: unknown;
}

const WINDOW_BUTTON_OPEN_POSITION = { x: 16, y: 17 };
const WINDOW_BUTTON_DEFAULT_DURATION = 360;

function cubicBezier(x1: number, y1: number, x2: number, y2: number) {
	function sampleCurveX(t: number) {
		return ((1 - 3 * x2 + 3 * x1) * t + (3 * x2 - 6 * x1)) * t * t + 3 * x1 * t;
	}

	function sampleCurveY(t: number) {
		return ((1 - 3 * y2 + 3 * y1) * t + (3 * y2 - 6 * y1)) * t * t + 3 * y1 * t;
	}

	function sampleCurveDerivativeX(t: number) {
		return (3 * (1 - 3 * x2 + 3 * x1) * t + 2 * (3 * x2 - 6 * x1)) * t + 3 * x1;
	}

	function solveCurveX(x: number) {
		let t = x;
		for (let i = 0; i < 8; i++) {
			const currentX = sampleCurveX(t) - x;
			if (Math.abs(currentX) < 0.000001) return t;
			const derivative = sampleCurveDerivativeX(t);
			if (Math.abs(derivative) < 0.000001) break;
			t -= currentX / derivative;
		}

		let start = 0;
		let end = 1;
		t = x;
		for (let i = 0; i < 12; i++) {
			const currentX = sampleCurveX(t);
			if (Math.abs(currentX - x) < 0.000001) return t;
			if (x > currentX) start = t;
			else end = t;
			t = (end - start) * 0.5 + start;
		}
		return t;
	}

	return (x: number) => {
		if (x <= 0) return 0;
		if (x >= 1) return 1;
		return sampleCurveY(solveCurveX(x));
	};
}

const sidebarChromeEase = cubicBezier(0.22, 1, 0.36, 1);

/** Coordinates macOS traffic-light placement and fullscreen renderer notifications. */
export function createWindowChrome(context: ShellWindowContext) {
	let animationTimer: ReturnType<typeof setTimeout> | null = null;
	let windowButtonPosition = { x: 16, y: 17 };

	function clearWindowButtonAnimation() {
		if (!animationTimer) return;
		clearTimeout(animationTimer);
		animationTimer = null;
	}

	function setWindowButtonPosition(position: { x: number; y: number }) {
		const mainWindow = context.getMainWindow();
		if (
			process.platform !== 'darwin' ||
			!mainWindow ||
			typeof mainWindow.setWindowButtonPosition !== 'function'
		) {
			return;
		}
		const next = { x: Math.round(position.x), y: Math.round(position.y) };
		mainWindow.setWindowButtonPosition(next);
		windowButtonPosition = next;
	}

	function setWindowButtonVisibility(visible: boolean) {
		const mainWindow = context.getMainWindow();
		if (
			process.platform !== 'darwin' ||
			!mainWindow ||
			typeof mainWindow.setWindowButtonVisibility !== 'function'
		) {
			return;
		}
		mainWindow.setWindowButtonVisibility(visible);
	}

	function animateWindowButtons(payload: boolean | SidebarState | undefined) {
		const mainWindow = context.getMainWindow();
		if (process.platform !== 'darwin' || !mainWindow) return;

		const sidebarState = typeof payload === 'object' && payload !== null ? payload : undefined;
		const open = typeof sidebarState?.open === 'boolean' ? sidebarState.open : Boolean(payload);
		clearWindowButtonAnimation();

		if (!open) {
			setWindowButtonVisibility(false);
			return;
		}

		setWindowButtonVisibility(true);
		const target = WINDOW_BUTTON_OPEN_POSITION;
		const rawDuration = Number(sidebarState?.duration);
		const duration = Number.isFinite(rawDuration)
			? Math.max(0, Math.min(rawDuration, 1000))
			: WINDOW_BUTTON_DEFAULT_DURATION;
		const start = { ...windowButtonPosition };

		if (duration <= 16 || (start.x === target.x && start.y === target.y)) {
			setWindowButtonPosition(target);
			return;
		}

		const startedAt = Date.now();
		const step = () => {
			if (!context.getMainWindow()) return;
			const progress = Math.min(1, (Date.now() - startedAt) / duration);
			const eased = sidebarChromeEase(progress);
			setWindowButtonPosition({
				x: start.x + (target.x - start.x) * eased,
				y: start.y + (target.y - start.y) * eased
			});

			if (progress < 1) {
				animationTimer = setTimeout(step, 16);
			} else {
				animationTimer = null;
				setWindowButtonPosition(target);
			}
		};
		step();
	}

	function sendFullScreenState() {
		const mainWindow = context.getMainWindow();
		if (!mainWindow || mainWindow.isDestroyed()) return;
		mainWindow.webContents.send('cometline:fullscreen-changed', mainWindow.isFullScreen());
	}

	return {
		animateWindowButtons,
		clearWindowButtonAnimation,
		sendFullScreenState,
		setWindowButtonPosition
	};
}
