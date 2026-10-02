import { goto } from '$app/navigation';
import { resolve } from '$app/paths';

export function currentPathname(): string {
	return typeof globalThis.location?.pathname === 'string' ? globalThis.location.pathname : '/';
}

export function isMiniRoutePath(pathname: string = currentPathname()): boolean {
	return pathname === '/mini' || pathname.startsWith('/mini/');
}

/** Opens a session in whichever shell (main or mini window) is currently showing. */
export function gotoSession(
	sessionId: string,
	pathname: string = currentPathname()
): Promise<void> {
	return goto(
		isMiniRoutePath(pathname)
			? resolve('/mini/session/[id]', { id: sessionId })
			: resolve('/session/[id]', { id: sessionId })
	);
}

/** Returns to the home route of whichever shell is currently showing. */
export function gotoHome(pathname: string = currentPathname()): Promise<void> {
	return goto(isMiniRoutePath(pathname) ? resolve('mini') : resolve('/'));
}
