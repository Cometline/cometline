export type GitGraphRefKind = 'branch' | 'remote' | 'tag' | 'head';

export type GitGraphRef = {
	name: string;
	kind: GitGraphRefKind;
	current?: boolean;
};

export type GitGraphLane = {
	lane: number;
	kind: 'commit' | 'pass' | 'merge';
	to?: number;
};

export type GitGraphCommit = {
	sha: string;
	short_sha: string;
	parents: string[];
	subject: string;
	author?: string;
	authored_at?: number;
	lane: number;
	lanes: GitGraphLane[];
	refs: GitGraphRef[];
};

export type GitGraphCommitFile = {
	path: string;
	status: 'modified' | 'renamed' | 'binary';
	additions: number;
	deletions: number;
};

export type GitGraphCommitDetail = {
	sha: string;
	short_sha: string;
	parents: string[];
	subject: string;
	body?: string;
	author?: string;
	author_email?: string;
	authored_at?: number;
	files: GitGraphCommitFile[];
};

export type GitGraphMenuAction =
	| 'checkout'
	| 'create-branch'
	| 'create-tag'
	| 'push-branch'
	| 'push-tag'
	| 'rename'
	| 'delete';

export type GitGraphMenuItem = {
	id: GitGraphMenuAction;
	label: string;
};

const laneColors = [
	'var(--accent)',
	'var(--status-success)',
	'var(--status-warning)',
	'#7c5cbf',
	'#0f766e',
	'#b45309'
];

export function laneColor(lane: number): string {
	return laneColors[((lane % laneColors.length) + laneColors.length) % laneColors.length];
}

export function isUnsafeRefName(name: string): boolean {
	const trimmed = name.trim();
	if (!trimmed || trimmed.toUpperCase() === 'HEAD') return true;
	if (trimmed.startsWith('-') || trimmed.includes('..')) return true;
	if (/\s/.test(trimmed)) return true;
	if (trimmed.includes('refs/')) return true;
	return false;
}

export function commitMenuItems(commit: GitGraphCommit, headSha: string): GitGraphMenuItem[] {
	const items: GitGraphMenuItem[] = [];
	if (commit.sha !== headSha) {
		items.push({ id: 'checkout', label: 'Checkout commit' });
	}
	items.push({ id: 'create-branch', label: 'Create branch here' });
	items.push({ id: 'create-tag', label: 'Create tag here' });
	return items;
}

export function refMenuItems(ref: GitGraphRef): GitGraphMenuItem[] {
	if (ref.kind === 'remote' || ref.kind === 'head') return [];
	if (ref.kind === 'tag') {
		return [
			{ id: 'push-tag', label: 'Push tag to remote' },
			{ id: 'rename', label: 'Rename tag' },
			{ id: 'delete', label: 'Delete tag' }
		];
	}
	const items: GitGraphMenuItem[] = [{ id: 'checkout', label: 'Checkout branch' }];
	if (ref.current) items.push({ id: 'push-branch', label: 'Push branch to remote' });
	items.push({ id: 'rename', label: 'Rename branch' });
	if (!ref.current) items.push({ id: 'delete', label: 'Delete branch' });
	return items;
}

export function dialogInputValue(
	dialog: {
		kind: string;
		name?: string;
		next?: string;
	} | null
): string {
	if (!dialog || dialog.kind.startsWith('delete')) return '';
	if (dialog.kind.startsWith('rename')) return dialog.next ?? '';
	if (dialog.kind === 'pick-remote') return '';
	return dialog.name ?? '';
}

export function defaultRemoteChoice(remotes: string[]): string {
	if (remotes.includes('origin')) return 'origin';
	return remotes[0] ?? '';
}

export function fileName(path: string): string {
	const parts = path.split('/');
	return parts[parts.length - 1] || path;
}

export function fileDir(path: string): string {
	const index = path.lastIndexOf('/');
	if (index <= 0) return '';
	return path.slice(0, index).replaceAll('/', ' / ');
}

export function formatGraphDate(unixSeconds?: number): string {
	if (!unixSeconds) return '';
	return new Intl.DateTimeFormat(undefined, {
		day: 'numeric',
		month: 'short',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).format(new Date(unixSeconds * 1000));
}

export type GraphStroke = {
	d: string;
	color: string;
};

export type GraphNode = {
	x: number;
	y: number;
	color: string;
};

const graphCell = 16;
const graphRow = 28;

export function graphRowHeight(): number {
	return graphRow;
}

function graphX(lane: number): number {
	return lane * graphCell + graphCell / 2;
}

function graphY(row: number, expansion?: { index: number; height: number }): number {
	const extra = expansion && row > expansion.index ? expansion.height : 0;
	return row * graphRow + graphRow / 2 + extra;
}

/** Continuous rails, the way Git Graph draws a branch as one path. */
export function graphStrokes(
	commits: GitGraphCommit[],
	includeWorking: boolean,
	expansion?: { index: number; height: number }
): GraphStroke[] {
	const strokes: GraphStroke[] = [];
	const columns = laneColumnCount(commits);
	const headLane = commits[0]?.lane ?? 0;
	if (includeWorking && commits.length > 0) {
		// Meet the next commit's circle. A rail that stops at this row's bottom
		// leaves a gap, because the next stroke starts at its own center.
		strokes.push({
			d: `M${graphX(headLane)},${graphRow / 2} L${graphX(headLane)},${graphY(1)}`,
			color: laneColor(headLane)
		});
	}
	const offset = includeWorking ? 1 : 0;
	const shift = expansion
		? { index: expansion.index + offset, height: expansion.height }
		: undefined;
	for (let lane = 0; lane < columns; lane++) {
		let start = -1;
		commits.forEach((commit, index) => {
			const active = commit.lanes.some((item) => item.lane === lane && item.to == null);
			if (active && start < 0) start = index;
			const ended = !active || index === commits.length - 1;
			if (!active || !ended || start < 0) return;
			const end = active ? index : index - 1;
			if (end >= start) {
				const y1 = graphY(start + offset, shift);
				const y2 = graphY(end + offset, shift);
				strokes.push({
					d: `M${graphX(lane)},${y1} L${graphX(lane)},${y2}`,
					color: laneColor(lane)
				});
			}
			start = -1;
		});
	}
	commits.forEach((commit, index) => {
		for (const join of commit.lanes) {
			if (join.to == null || join.to === join.lane) continue;
			const x1 = graphX(join.lane);
			const x2 = graphX(join.to);
			const y = graphY(index + offset, shift);
			strokes.push({
				d: `M${x1},${y} C${(x1 + x2) / 2},${y} ${x2},${y - graphRow * 0.65} ${x2},${y}`,
				color: laneColor(join.to)
			});
		}
	});
	return strokes;
}

/** Commit and working-tree circles, in the same coordinates as graphStrokes. */
export function graphNodes(
	commits: GitGraphCommit[],
	includeWorking: boolean,
	expansion?: { index: number; height: number }
): GraphNode[] {
	const nodes: GraphNode[] = [];
	const offset = includeWorking ? 1 : 0;
	const shift = expansion
		? { index: expansion.index + offset, height: expansion.height }
		: undefined;
	if (includeWorking && commits.length > 0) {
		nodes.push({
			x: graphX(commits[0].lane),
			y: graphRow / 2,
			color: laneColor(commits[0].lane)
		});
	}
	commits.forEach((commit, index) => {
		nodes.push({
			x: graphX(commit.lane),
			y: graphY(index + offset, shift),
			color: laneColor(commit.lane)
		});
	});
	return nodes;
}

export function laneColumnCount(commits: GitGraphCommit[]): number {
	let max = 1;
	for (const commit of commits) {
		max = Math.max(max, commit.lane + 1);
		for (const cell of commit.lanes) max = Math.max(max, cell.lane + 1);
	}
	return max;
}
