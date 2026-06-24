import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [sveltekit()],
	resolve: {
		conditions: process.env.VITEST ? ['browser'] : []
	},
	test: {
		globals: true,
		environment: 'node',
		include: ['src/lib/**/*.test.ts', 'src/lib/**/*.svelte.test.ts'],
		environmentMatchGlobs: [['**/*.svelte.test.ts', 'jsdom']],
		setupFiles: ['./src/test-setup.ts'],
		coverage: {
			provider: 'v8',
			include: ['src/lib/**'],
			exclude: ['src/lib/generated/**']
		}
	}
});
