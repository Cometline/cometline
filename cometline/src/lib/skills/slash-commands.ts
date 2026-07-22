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
		description: 'Draft a new Agent Skill for review in Skill Drafts'
	},
	{
		name: 'model',
		description: 'Switch the model for this session'
	},
	{
		name: 'job',
		description: 'Claim a ready job and start working on it'
	}
];

export function expandCreateSkillCommand(userText: string): string {
	const rest = userText.trim();
	let prompt =
		'Draft a new Agent Skill for CometMind.\n\n' +
		'Target directory: ~/.cometmind/skill-drafts/{skill-name}/\n\n' +
		'Requirements:\n' +
		'1. Use the `write_skill_draft` tool to create SKILL.md (YAML frontmatter with name and description, then markdown body).\n' +
		'2. If the user did not provide a detailed request, infer the draft from the current session context or the current completed job being discussed.\n' +
		'3. Follow Agent Skills conventions: clear trigger scenarios, step-by-step workflow, examples, and constraints.\n' +
		'4. Skill names use lowercase letters, numbers, and hyphens only.\n' +
		'5. If there is not enough reusable signal yet, explain that instead of forcing a draft.\n' +
		'6. After writing, summarize the draft name, what it does, and that it can be edited or promoted from Skill Drafts.';
	if (rest) {
		prompt += '\n\nUser request:\n' + rest;
	}
	return prompt;
}

export function expandBuiltinSlashCommand(
	text: string
): { text: string; displayText?: string } | null {
	const match = /^\s*\/([\w-]+)(?:\s+([\s\S]*))?$/.exec(text);
	if (!match) return null;
	const name = match[1];
	const builtin = BUILTIN_SLASH_COMMANDS.find((cmd) => cmd.name === name);
	if (!builtin) return null;
	const rest = match[2]?.trimStart() ?? '';
	if (name === 'create-skill') {
		const displayText = rest ? `/create-skill ${rest}` : '/create-skill';
		return { text: expandCreateSkillCommand(rest), displayText };
	}
	return null;
}

export function parseJobCommand(text: string): { query: string } | null {
	// Require a trailing space so bare `/job` stays in slash autocomplete until Enter.
	const match = /^\s*\/job(\s+(.*))?$/i.exec(text);
	if (!match || match[1] === undefined) return null;
	return { query: (match[2] ?? '').trim() };
}

export function filterJobOptions<T extends { id: string; description: string }>(
	query: string,
	jobs: T[]
): T[] {
	const q = query.toLowerCase();
	return jobs.filter((job) => {
		if (!q) return true;
		return job.id.toLowerCase().includes(q) || job.description.toLowerCase().includes(q);
	});
}

export function parseChangeCommand(text: string): { query: string } | null {
	// Require a trailing space so bare `/change` stays in slash autocomplete until Enter.
	const match = /^\s*\/change(\s+(.*))?$/i.exec(text);
	if (!match || match[1] === undefined) return null;
	return { query: (match[2] ?? '').trim() };
}

export function parseClearCommand(text: string): boolean {
	return /^\s*\/clear\s*$/i.test(text);
}

export function isClearCommand(text: string): boolean {
	return parseClearCommand(text);
}

export function parseModelCommand(text: string): { query: string } | null {
	// Require a trailing space so bare `/model` stays in slash autocomplete until Enter.
	const match = /^\s*\/model(\s+(.*))?$/i.exec(text);
	if (!match || match[1] === undefined) return null;
	return { query: (match[2] ?? '').trim() };
}

export function isChangeWorkspaceCommand(text: string): boolean {
	return /^\s*\/change(?:\s.*)?$/i.test(text);
}

export function isModelCommand(text: string): boolean {
	return /^\s*\/model(?:\s.*)?$/i.test(text);
}

export function isJobCommand(text: string): boolean {
	return /^\s*\/job(?:\s.*)?$/i.test(text);
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

export function scoreSlashMenuMatch(query: string, name: string, description: string): number {
	const q = query.toLowerCase();
	if (!q) return 4;
	const n = name.toLowerCase();
	const d = description.toLowerCase();
	if (n === q) return 4;
	if (n.startsWith(q)) return 3;
	if (n.includes(q)) return 2;
	if (d.includes(q)) return 1;
	return 0;
}

export function filterSlashMenuOptions(
	query: string,
	skills: { name: string; description: string }[]
): SlashMenuOption[] {
	const q = query.toLowerCase();
	const scoredBuiltins = BUILTIN_SLASH_COMMANDS.map((cmd) => ({
		cmd,
		score: scoreSlashMenuMatch(q, cmd.name, cmd.description)
	})).filter((item) => item.score > 0);
	const scoredSkills = skills
		.map((skill) => ({
			skill,
			score: scoreSlashMenuMatch(q, skill.name, skill.description)
		}))
		.filter((item) => item.score > 0);

	// When the query strongly matches a name, drop description-only hits so
	// typing `/change` does not keep unrelated skills that merely mention "change".
	const bestScore = Math.max(
		0,
		...scoredBuiltins.map((item) => item.score),
		...scoredSkills.map((item) => item.score)
	);
	const minScore = bestScore >= 3 ? 2 : 1;
	const builtins = scoredBuiltins
		.filter((item) => item.score >= minScore)
		.sort((a, b) => b.score - a.score || a.cmd.name.localeCompare(b.cmd.name))
		.map((item) => ({
			kind: 'builtin' as const,
			name: item.cmd.name,
			description: item.cmd.description
		}));
	const skillOptions = scoredSkills
		.filter((item) => item.score >= minScore)
		.sort((a, b) => b.score - a.score || a.skill.name.localeCompare(b.skill.name))
		.map((item) => ({
			kind: 'skill' as const,
			name: item.skill.name,
			description: item.skill.description
		}));
	return [...builtins, ...skillOptions];
}
