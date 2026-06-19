import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		// Bind to the IPv4 loopback explicitly. Vite v8 defaults to the
		// `localhost` hostname, which resolves to IPv6 `[::1]` on macOS. The
		// Electron dev launcher waits on `http://127.0.0.1:5173` (see
		// package.json `dev:electron`) and the main process loads that same
		// URL (electron/main.cjs), so a `[::1]`-only bind makes `wait-on`
		// hang forever, Electron never launches, and the CometMind sidecar is
		// never spawned — leaving the renderer stuck with no backend.
		host: '127.0.0.1',
		port: 5173,
		strictPort: true
	}
});
