export type AppToastTone = 'success' | 'warning' | 'error';

export interface AppToast {
	id: string;
	label: string;
	detail: string;
	tone: AppToastTone;
	onOpen?: () => void;
}

const TOAST_DURATION_MS = 5000;
const MAX_TOASTS = 3;

function createAppToastStore() {
	let toasts = $state<AppToast[]>([]);
	const timers = new Map<string, ReturnType<typeof setTimeout>>();

	function dismiss(id: string) {
		const timer = timers.get(id);
		if (timer) clearTimeout(timer);
		timers.delete(id);
		toasts = toasts.filter((toast) => toast.id !== id);
	}

	function push(label: string, detail = '', tone: AppToastTone = 'success', onOpen?: () => void) {
		const toast: AppToast = {
			id: `app-toast-${Date.now()}-${Math.random().toString(36).slice(2)}`,
			label,
			detail: detail.replace(/\s+/g, ' ').trim(),
			tone,
			onOpen
		};
		toasts = [...toasts, toast].slice(-MAX_TOASTS);
		timers.set(
			toast.id,
			setTimeout(() => dismiss(toast.id), TOAST_DURATION_MS)
		);
	}

	function success(label: string, detail = '', onOpen?: () => void) {
		push(label, detail, 'success', onOpen);
	}

	function warning(label: string, detail = '', onOpen?: () => void) {
		push(label, detail, 'warning', onOpen);
	}

	function error(label: string, detail = '', onOpen?: () => void) {
		push(label, detail, 'error', onOpen);
	}

	return {
		get toasts() {
			return toasts;
		},
		success,
		warning,
		error,
		dismiss
	};
}

export const appToastStore = createAppToastStore();
