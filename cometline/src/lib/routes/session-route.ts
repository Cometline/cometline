export function currentPathname(): string {
	return typeof globalThis.location?.pathname === 'string' ? globalThis.location.pathname : '/';
}

export function isMiniRoutePath(pathname: string = currentPathname()): boolean {
	return pathname === '/mini' || pathname.startsWith('/mini/');
}

export function sessionRouteFor(sessionId: string, pathname: string = currentPathname()): string {
	return isMiniRoutePath(pathname) ? `/mini/session/${sessionId}` : `/session/${sessionId}`;
}

export function homeRouteFor(pathname: string = currentPathname()): string {
	return isMiniRoutePath(pathname) ? '/mini' : '/';
}
