import * as esbuild from 'esbuild';

await esbuild.build({
	entryPoints: ['src/lib/settings/schema.ts'],
	bundle: true,
	platform: 'node',
	format: 'cjs',
	outfile: 'electron/settings-schema.cjs',
	logLevel: 'info'
});
