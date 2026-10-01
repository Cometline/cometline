export function screenCaptureStatusLabel(status: string): string {
	switch (status) {
		case 'granted':
			return 'System permission: granted';
		case 'denied':
			return 'System permission: denied — open System Settings to allow Cometline';
		case 'not-determined':
			return 'System permission: not determined yet';
		case 'restricted':
			return 'System permission: restricted by system policy';
		case 'unsupported':
			return 'System permission: managed by the OS on this platform';
		default:
			return 'System permission: unknown';
	}
}

function numberFromInput(event: Event): number {
	return Number((event.currentTarget as HTMLInputElement).value);
}

export function miniWindowTimeoutFromInput(event: Event): number {
	const value = numberFromInput(event);
	return Number.isFinite(value) ? Math.min(24 * 60, Math.max(1, Math.floor(value))) : 30;
}

export function nonNegativeIntFromInput(event: Event): number {
	const value = numberFromInput(event);
	return Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0;
}

export function positiveIntFromInput(event: Event): number {
	const value = numberFromInput(event);
	return Number.isFinite(value) ? Math.max(1, Math.floor(value)) : 1;
}
