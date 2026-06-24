import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import svelteParser from 'svelte-eslint-parser';
import prettier from 'eslint-config-prettier';
import globals from 'globals';

export default [
	js.configs.recommended,
	...tseslint.configs.recommended,
	{
		ignores: [
			'.svelte-kit/',
			'build/',
			'buildResources/',
			'dist/',
			'electron/settings-schema.cjs',
			'node_modules/',
			'pnpm-lock.yaml',
			'static/',
			'storybook-static/',
			'*.config.{js,ts,cjs,mjs}',
			'coverage/'
		]
	},
	{
		files: ['**/*.svelte'],
		plugins: {
			svelte
		},
		languageOptions: {
			parser: svelteParser,
			parserOptions: {
				parser: tseslint.parser,
				extraFileExtensions: ['.svelte'],
				svelteConfig: {
					runes: true
				}
			},
			globals: {
				...globals.browser,
				...globals.node
			}
		},
		rules: {
			...svelte.configs['flat/recommended'][0].rules,
			...svelte.configs['flat/prettier'][0].rules
		}
	},
	{
		files: ['**/*.{js,ts,cjs,mjs}'],
		languageOptions: {
			globals: {
				...globals.browser,
				...globals.node
			}
		}
	},
	{
		files: ['**/*.cjs'],
		languageOptions: {
			sourceType: 'commonjs',
			globals: {
				...globals.node
			}
		},
		rules: {
			'@typescript-eslint/no-require-imports': 'off',
			'@typescript-eslint/no-unused-vars': [
				'error',
				{
					argsIgnorePattern: '^_',
					varsIgnorePattern: '^_',
					caughtErrorsIgnorePattern: '^_'
				}
			]
		}
	},
	prettier,
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
