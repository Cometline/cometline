export function isAbortError(err: unknown) {
	return err instanceof DOMException && err.name === 'AbortError';
}

export function isRunConflict(err: unknown) {
	return Boolean(err && typeof err === 'object' && 'status' in err && err.status === 409);
}

export function isSessionRunningConflict(err: unknown) {
	return isConflictCode(err, 'session_running');
}

export function isSessionCompactingConflict(err: unknown) {
	return isConflictCode(err, 'session_compacting');
}

function isConflictCode(err: unknown, code: string) {
	return Boolean(
		err &&
		typeof err === 'object' &&
		'status' in err &&
		err.status === 409 &&
		'code' in err &&
		err.code === code
	);
}
