export interface BuiltinSlashCommand {
	name: string;
	description: string;
}

export const BUILTIN_SLASH_COMMANDS: BuiltinSlashCommand[] = [
	{
		name: 'change',
		description: 'Fork this session into another workspace directory'
	},
	{
		name: 'clear',
		description: 'Clear transcript history and start fresh in this session'
	},
	{
		name: 'create-skill',
		description: 'Build a new Agent Skill in ~/.cometmind/skills'
	},
	{
		name: 'model',
		description: 'Switch the model for this session'
	},
	{
		name: 'job',
		description: 'Claim a ready job and start working on it'
	},
	{
		name: 'list-jobs',
		description: 'List ready jobs in the queue'
	}
];

export function expandCreateSkillCommand(userText: string): string {
	const rest = userText.trim();
	let prompt =
		'Create a new Agent Skill for CometMind.\n\n' +
		'Target directory: ~/.cometmind/skills/{skill-name}/\n\n' +
		'Requirements:\n' +
		'1. Use the `write_skill` tool to create SKILL.md (YAML frontmatter with name and description, then markdown body).\n' +
		'2. Follow Agent Skills conventions: clear trigger scenarios, step-by-step workflow, examples, and constraints.\n' +
		'3. Skill names use lowercase letters, numbers, and hyphens only.\n' +
		'4. If the request is vague, ask up to two clarifying questions before writing.\n' +
		'5. After writing, summarize the skill name, what it does, and how to invoke it with /{skill-name} in Cometline.';
	if (rest) {
		prompt += '\n\nUser request:\n' + rest;
	}
	return prompt;
}

export function expandBuiltinSlashCommand(text: string): string | null {
	const match = /^\s*\/([\w-]+)(?:\s+([\s\S]*))?$/.exec(text);
	if (!match) return null;
	const name = match[1];
	const builtin = BUILTIN_SLASH_COMMANDS.find((cmd) => cmd.name === name);
	if (!builtin) return null;
	const rest = match[2]?.trimStart() ?? '';
	if (name === 'create-skill') {
		return expandCreateSkillCommand(rest);
	}
	if (name === 'change') {
		return null;
	}
	if (name === 'clear') {
		return null;
	}
	if (name === 'model') {
		return null;
	}
	if (name === 'job') {
		return null;
	}
	if (name === 'list-jobs') {
		return null;
	}
	return null;
}

export function parseJobCommand(text: string): { query: string } | null {
	const match = /^\s*\/job(?:\s+(.*))?$/i.exec(text);
	if (!match) return null;
	return { query: (match[1] ?? '').trim() };
}

export function parseListJobsCommand(text: string): boolean {
	return /^\s*\/list-jobs\s*$/i.test(text);
}

export function filterJobOptions<T extends { id: string; description: string }>(query: string, jobs: T[]): T[] {
	const q = query.toLowerCase();
	return jobs.filter((job) => {
		if (!q) return true;
		return (
			job.id.toLowerCase().includes(q) ||
			job.description.toLowerCase().includes(q)
		);
	});
}

export function parseChangeCommand(text: string): { query: string } | null {
	const match = /^\s*\/change(?:\s+(.*))?$/i.exec(text);
	if (!match) return null;
	return { query: (match[1] ?? '').trim() };
}

export function parseClearCommand(text: string): boolean {
	return /^\s*\/clear\s*$/i.test(text);
}

export function isClearCommand(text: string): boolean {
	return parseClearCommand(text);
}

export function parseModelCommand(text: string): { query: string } | null {
	const match = /^\s*\/model(?:\s+(.*))?$/i.exec(text);
	if (!match) return null;
	return { query: (match[1] ?? '').trim() };
}

export function isChangeWorkspaceCommand(text: string): boolean {
	return parseChangeCommand(text) !== null;
}

export type WorkspaceMenuOption =
	| { kind: 'workspace'; path: string; label: string; description: string; deletable: boolean }
	| { kind: 'browse'; path: ''; label: string; description: string };

export function workspaceLabel(path: string): string {
	const parts = path.split(/[/\\]/).filter(Boolean);
	return parts[parts.length - 1] || path;
}

export function normalizeWorkspacePath(path: string): string {
	return path.trim().replace(/\\/g, '/').replace(/\/+$/, '') || path.trim();
}

function sessionCountForPath(path: string, sessionCountByPath: Map<string, number>): number {
	const normalized = normalizeWorkspacePath(path);
	const direct = sessionCountByPath.get(path) ?? sessionCountByPath.get(normalized);
	if (direct !== undefined) return direct;
	for (const [key, count] of sessionCountByPath) {
		if (normalizeWorkspacePath(key) === normalized) return count;
	}
	return 0;
}

export function filterWorkspaceOptions(
	query: string,
	paths: string[],
	sessionCountByPath: Map<string, number> = new Map()
): WorkspaceMenuOption[] {
	const q = query.toLowerCase();
	const filtered = paths.filter((path) => {
		if (!q) return true;
		const lower = path.toLowerCase();
		return lower.includes(q) || workspaceLabel(path).toLowerCase().includes(q);
	});
	const options: WorkspaceMenuOption[] = filtered.map((path) => ({
		kind: 'workspace',
		path,
		label: workspaceLabel(path),
		description: path,
		deletable: sessionCountForPath(path, sessionCountByPath) === 0
	}));
	options.push({
		kind: 'browse',
		path: '',
		label: 'Browse folder…',
		description: 'Open the native folder picker'
	});
	return options;
}

export type SlashMenuOption =
	| { kind: 'builtin'; name: string; description: string }
	| { kind: 'skill'; name: string; description: string };

export function filterSlashMenuOptions(
	query: string,
	skills: { name: string; description: string }[]
): SlashMenuOption[] {
	const q = query.toLowerCase();
	const scoreMatch = (name: string, description: string): number => {
		const n = name.toLowerCase();
		const d = description.toLowerCase();
		if (n.startsWith(q)) return 3;
		if (n.includes(q)) return 2;
		if (d.includes(q)) return 1;
		return 0;
	};
	const builtins = BUILTIN_SLASH_COMMANDS.map((cmd) => ({
		cmd,
		score: q ? scoreMatch(cmd.name, cmd.description) : 4
	}))
		.filter((item) => item.score > 0)
		.sort((a, b) => b.score - a.score || a.cmd.name.localeCompare(b.cmd.name))
		.map((item) => ({
			kind: 'builtin' as const,
			name: item.cmd.name,
			description: item.cmd.description
		}));
	const skillOptions = skills
		.map((skill) => ({ skill, score: q ? scoreMatch(skill.name, skill.description) : 4 }))
		.filter((item) => item.score > 0)
		.sort((a, b) => b.score - a.score || a.skill.name.localeCompare(b.skill.name))
		.map((item) => ({
			kind: 'skill' as const,
			name: item.skill.name,
			description: item.skill.description
		}));
	return [...builtins, ...skillOptions];
}
