import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export const SESSION_ID_RE = /^[A-Za-z0-9_-]{1,128}$/;

export type ShellKind = 'zsh' | 'bash' | 'other';

export function isValidSessionId(id: string) {
	return SESSION_ID_RE.test(id);
}

export function cometmindDataDir(env: NodeJS.ProcessEnv = process.env, home = os.homedir()) {
	const override = env.COMETMIND_DATA_DIR?.trim();
	if (override) return override;
	return path.join(home, '.cometmind');
}

export function terminalEnvDir(
	sessionId: string,
	env: NodeJS.ProcessEnv = process.env,
	home = os.homedir()
) {
	if (!isValidSessionId(sessionId)) throw new Error('Invalid terminal session id');
	return path.join(cometmindDataDir(env, home), 'terminal-env', sessionId);
}

export function shellIntegrationRoot(packaged: boolean, resourcesPath: string, appPath: string) {
	if (packaged) return path.join(resourcesPath, 'shell-integration');
	return path.join(appPath, 'electron/resources/shell-integration');
}

export function shellKind(shellPath: string): ShellKind {
	const name = path.basename(shellPath).toLowerCase();
	if (name === 'zsh' || name.endsWith('zsh')) return 'zsh';
	if (name === 'bash' || name.endsWith('bash')) return 'bash';
	return 'other';
}

export function integrationScriptPath(root: string, shellPath: string) {
	switch (shellKind(shellPath)) {
		case 'zsh':
			return path.join(root, 'zsh', 'integration.zsh');
		case 'bash':
			return path.join(root, 'bash', 'integration.bash');
		default:
			return '';
	}
}

export function zshDotDir(envDir: string) {
	return path.join(envDir, 'zdot');
}

export function bashRcPath(envDir: string) {
	return path.join(envDir, 'bashrc');
}

export function shSingleQuote(value: string) {
	return `'${value.replace(/'/g, `'\\''`)}'`;
}

function sourceUserZshFile(filename: string, restoreOurZdot: boolean) {
	const restore = restoreOurZdot
		? `if [[ -n "\${COMETLINE_ZDOTDIR-}" ]]; then
	ZDOTDIR="\${COMETLINE_ZDOTDIR}"
fi
`
		: `if [[ -n "\${COMETLINE_USER_ZDOTDIR-}" ]]; then
	ZDOTDIR="\${COMETLINE_USER_ZDOTDIR}"
else
	unset ZDOTDIR
fi
`;
	return `_cometline_user_zdotdir="\${COMETLINE_USER_ZDOTDIR:-\${HOME}}"
if [[ -f "\${_cometline_user_zdotdir}/${filename}" ]]; then
	ZDOTDIR="\${_cometline_user_zdotdir}"
	source "\${_cometline_user_zdotdir}/${filename}"
fi
${restore}unset _cometline_user_zdotdir
`;
}

export function writeZshDotDir(envDir: string, integrationPath: string) {
	const zdot = zshDotDir(envDir);
	fs.mkdirSync(zdot, { recursive: true, mode: 0o700 });
	const quoted = shSingleQuote(integrationPath);
	const sourceIntegration = `[[ -f ${quoted} ]] && source ${quoted}\n`;
	fs.writeFileSync(
		path.join(zdot, '.zshenv'),
		`if [[ -n "\${COMETLINE_ZSHENV_SOURCED-}" ]]; then
	return
fi
COMETLINE_ZSHENV_SOURCED=1
: "\${COMETLINE_ZDOTDIR:=\${ZDOTDIR:-}}"
if [[ -n "\${COMETLINE_USER_ZDOTDIR-}" && -f "\${COMETLINE_USER_ZDOTDIR}/.zshenv" ]]; then
	ZDOTDIR="\${COMETLINE_USER_ZDOTDIR}"
	source "\${COMETLINE_USER_ZDOTDIR}/.zshenv"
elif [[ -z "\${COMETLINE_USER_ZDOTDIR-}" && -f "\${HOME}/.zshenv" ]]; then
	unset ZDOTDIR
	source "\${HOME}/.zshenv"
fi
if [[ -n "\${COMETLINE_ZDOTDIR-}" ]]; then
	ZDOTDIR="\${COMETLINE_ZDOTDIR}"
fi
`,
		{ mode: 0o600 }
	);
	fs.writeFileSync(path.join(zdot, '.zprofile'), sourceUserZshFile('.zprofile', true), {
		mode: 0o600
	});
	fs.writeFileSync(
		path.join(zdot, '.zshrc'),
		`${sourceIntegration}${sourceUserZshFile('.zshrc', false)}${sourceIntegration}`,
		{ mode: 0o600 }
	);
	fs.writeFileSync(path.join(zdot, '.zlogin'), sourceUserZshFile('.zlogin', false), {
		mode: 0o600
	});
	return zdot;
}

export function writeBashRc(envDir: string, integrationPath: string) {
	const rc = bashRcPath(envDir);
	fs.mkdirSync(envDir, { recursive: true, mode: 0o700 });
	const quoted = shSingleQuote(integrationPath);
	fs.writeFileSync(
		rc,
		`if [ -f /etc/profile ]; then . /etc/profile; fi
if [ -f "$HOME/.bash_profile" ]; then . "$HOME/.bash_profile"
elif [ -f "$HOME/.bash_login" ]; then . "$HOME/.bash_login"
elif [ -f "$HOME/.profile" ]; then . "$HOME/.profile"
fi
if [ -f ${quoted} ]; then . ${quoted}; fi
`,
		{ mode: 0o600 }
	);
	return rc;
}

export function prepareEnvDir(
	sessionId: string,
	env: NodeJS.ProcessEnv = process.env,
	home = os.homedir()
) {
	const dir = terminalEnvDir(sessionId, env, home);
	fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
	return dir;
}

export function removeTerminalEnvDir(
	sessionId: string,
	env: NodeJS.ProcessEnv = process.env,
	home = os.homedir()
) {
	if (!isValidSessionId(sessionId)) return;
	fs.rmSync(terminalEnvDir(sessionId, env, home), { force: true, recursive: true });
}

export function clearAllTerminalEnv(env: NodeJS.ProcessEnv = process.env, home = os.homedir()) {
	const root = path.join(cometmindDataDir(env, home), 'terminal-env');
	fs.rmSync(root, { force: true, recursive: true });
}

export function spawnArgs(kind: ShellKind, bashRc: string | null): string[] {
	if (kind === 'bash' && bashRc) return ['-i', '--rcfile', bashRc];
	return ['-l'];
}
