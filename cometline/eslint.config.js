import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import prettier from 'eslint-config-prettier';
import globals from 'globals';

export default [
	js.configs.recommended,
	...tseslint.configs.recommended,
	...svelte.configs['flat/recommended'],
	prettier,
	...svelte.configs['flat/prettier'],
	{
		ignores: [
			'.svelte-kit/',
			'build/',
			'buildResources/',
			'dist/',
			'electron/dist/',
			'node_modules/',
			'pnpm-lock.yaml',
			'static/',
			'storybook-static/',
			'*.config.{js,ts,cjs,mjs}',
			'coverage/'
		]
	},
	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: {
			parserOptions: {
				parser: tseslint.parser,
				extraFileExtensions: ['.svelte'],
				svelteConfig: {
					runes: true
				}
			}
		}
	},
	{
		// Controllers keep private Map/Set bookkeeping and publish changes by reassigning $state.
		files: ['**/*.svelte.ts'],
		rules: {
			'svelte/prefer-svelte-reactivity': 'off'
		}
	},
	{
		languageOptions: {
			globals: {
				...globals.browser,
				...globals.node
			}
		}
	},
	{
		rules: {
			'@typescript-eslint/no-unused-vars': [
				'error',
				{
					argsIgnorePattern: '^_',
					varsIgnorePattern: '^_',
					caughtErrorsIgnorePattern: '^_'
				}
			],
			'no-undef': 'off'
		}
	}
];
