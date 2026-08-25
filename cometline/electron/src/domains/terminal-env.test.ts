import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';

import {
	bashRcPath,
	clearAllTerminalEnv,
	cometmindDataDir,
	integrationScriptPath,
	isValidSessionId,
	prepareEnvDir,
	shellIntegrationRoot,
	shellKind,
	shSingleQuote,
	spawnArgs,
	terminalEnvDir,
	writeBashRc,
	writeZshDotDir,
	zshDotDir
} from './terminal-env.js';

const temporaryDirectories: string[] = [];

afterEach(() => {
	for (const directory of temporaryDirectories.splice(0)) {
		fs.rmSync(directory, { force: true, recursive: true });
	}
});

describe('terminal-env', () => {
	it('accepts path-safe session ids', () => {
		expect(isValidSessionId('01ARZ3NDEKTSV4RRFFQ69G5FAV')).toBe(true);
		expect(isValidSessionId('../etc')).toBe(false);
		expect(isValidSessionId('a/b')).toBe(false);
	});

	it('uses COMETMIND_DATA_DIR when set', () => {
		expect(cometmindDataDir({ COMETMIND_DATA_DIR: '/tmp/state' }, '/Users/me')).toBe(
			'/tmp/state'
		);
		expect(cometmindDataDir({}, '/Users/me')).toBe(path.join('/Users/me', '.cometmind'));
	});

	it('prepares a session env directory', () => {
		const home = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-term-env-'));
		temporaryDirectories.push(home);
		const env = { COMETMIND_DATA_DIR: path.join(home, 'state') };
		const dir = prepareEnvDir('sess1', env, home);
		expect(fs.statSync(dir).isDirectory()).toBe(true);
	});

	it('writes zsh wrappers that source the integration script', () => {
		const home = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-term-env-'));
		temporaryDirectories.push(home);
		const envDir = path.join(home, 'env');
		fs.mkdirSync(envDir);
		const script = path.join(home, 'integration.zsh');
		fs.writeFileSync(script, '');
		const zdot = writeZshDotDir(envDir, script);
		expect(zdot).toBe(zshDotDir(envDir));
		const zshrc = fs.readFileSync(path.join(zdot, '.zshrc'), 'utf8');
		expect(zshrc).toContain(`source '${script}'`);
		expect(fs.existsSync(path.join(zdot, '.zshenv'))).toBe(true);
		expect(fs.readFileSync(path.join(zdot, '.zprofile'), 'utf8')).toContain('.zprofile');
	});

	it('writes a bash rc wrapper instead of a typed source command', () => {
		const home = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-term-env-'));
		temporaryDirectories.push(home);
		const envDir = path.join(home, 'env');
		const script = path.join(home, 'integration.bash');
		fs.writeFileSync(script, '');
		const rc = writeBashRc(envDir, script);
		expect(rc).toBe(bashRcPath(envDir));
		expect(fs.readFileSync(rc, 'utf8')).toContain(`. '${script}'`);
		expect(spawnArgs('bash', rc)).toEqual(['-i', '--rcfile', rc]);
		expect(spawnArgs('zsh', null)).toEqual(['-l']);
	});

	it('quotes shell paths', () => {
		expect(shSingleQuote("it's")).toBe(`'it'\\''s'`);
	});

	it('resolves zsh vs bash integration scripts', () => {
		expect(shellKind('/bin/zsh')).toBe('zsh');
		expect(shellKind('/opt/homebrew/bin/bash')).toBe('bash');
		expect(integrationScriptPath('/res', '/bin/zsh')).toBe(
			path.join('/res', 'zsh', 'integration.zsh')
		);
		expect(integrationScriptPath('/res', '/bin/bash')).toBe(
			path.join('/res', 'bash', 'integration.bash')
		);
		expect(integrationScriptPath('/res', '/bin/fish')).toBe('');
		expect(shellIntegrationRoot(true, '/Resources', '/app')).toBe(
			path.join('/Resources', 'shell-integration')
		);
		expect(shellIntegrationRoot(false, '/Resources', '/app')).toBe(
			path.join('/app', 'electron/resources/shell-integration')
		);
	});

	it('clears all session env dirs', () => {
		const home = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-term-env-'));
		temporaryDirectories.push(home);
		const env = { COMETMIND_DATA_DIR: path.join(home, 'state') };
		prepareEnvDir('sess1', env, home);
		clearAllTerminalEnv(env, home);
		expect(fs.existsSync(terminalEnvDir('sess1', env, home))).toBe(false);
	});
});
