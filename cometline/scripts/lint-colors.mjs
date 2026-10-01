import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

const root = new URL('../src', import.meta.url).pathname;
const allowlist = new Set(['app.css']);
const hex = /#[0-9a-fA-F]{3,8}\b/;

/** @type {string[]} */
const violations = [];

function walk(dir) {
	for (const entry of readdirSync(dir)) {
		const path = join(dir, entry);
		const stat = statSync(path);
		if (stat.isDirectory()) {
			walk(path);
			continue;
		}
		if (!/\.(css|svelte)$/.test(entry)) continue;
		const rel = relative(root, path);
		if (allowlist.has(rel)) continue;
		const content = readFileSync(path, 'utf8');
		const lines = content.split('\n');
		for (let index = 0; index < lines.length; index += 1) {
			if (hex.test(lines[index])) {
				violations.push(`${rel}:${index + 1}: ${lines[index].trim()}`);
			}
		}
	}
}

walk(root);

if (violations.length > 0) {
	console.error('Raw hex colors found outside app.css:\n');
	for (const line of violations) console.error(`  ${line}`);
	process.exit(1);
}

console.log('No raw hex colors outside app.css.');
