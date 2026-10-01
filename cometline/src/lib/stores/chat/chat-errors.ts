export function isAbortError(err: unknown) {
	return err instanceof DOMException && err.name === 'AbortError';
}

export function isRunConflict(err: unknown) {
	return Boolean(err && typeof err === 'object' && 'status' in err && err.status === 409);
}

export function isSessionRunningConflict(err: unknown) {
	return Boolean(
		err &&
		typeof err === 'object' &&
		'status' in err &&
		err.status === 409 &&
		'code' in err &&
		err.code === 'session_running'
	);
}
