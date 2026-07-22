import * as esbuild from 'esbuild';

const watch = process.argv.includes('--watch');
const shared = {
	bundle: true,
	packages: 'external',
	platform: 'node',
	target: 'node22',
	sourcemap: true,
	logLevel: 'info'
};

const builds = [
	{
		...shared,
		entryPoints: ['electron/src/main.ts'],
		format: 'esm',
		outfile: 'electron/dist/main.js'
	},
	{
		...shared,
		entryPoints: ['electron/src/preload.ts'],
		format: 'cjs',
		outfile: 'electron/dist/preload.cjs'
	}
];

if (watch) {
	await Promise.all(
		builds.map((options) => esbuild.context(options).then((context) => context.watch()))
	);
} else {
	await Promise.all(builds.map((options) => esbuild.build(options)));
}
