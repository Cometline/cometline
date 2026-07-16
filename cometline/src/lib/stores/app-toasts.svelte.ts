export interface AppToast {
	id: string;
	label: string;
	detail: string;
}

const TOAST_DURATION_MS = 2500;
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

	function success(label: string, detail = '') {
		const toast: AppToast = {
			id: `app-toast-${Date.now()}-${Math.random().toString(36).slice(2)}`,
			label,
			detail: detail.replace(/\s+/g, ' ').trim()
		};
		toasts = [...toasts, toast].slice(-MAX_TOASTS);
		timers.set(
			toast.id,
			setTimeout(() => dismiss(toast.id), TOAST_DURATION_MS)
		);
	}

	return {
		get toasts() {
			return toasts;
		},
		success,
		dismiss
	};
}

export const appToastStore = createAppToastStore();
