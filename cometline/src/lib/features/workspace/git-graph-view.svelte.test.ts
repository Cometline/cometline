import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createGitGraphViewController } from './git-graph-view.svelte.js';
import {
	fetchWorkspaceGit,
	getWorkspaceGitGraph,
	getWorkspaceGitStatus,
	listWorkspaceGitRemotes,
	pushWorkspaceGit,
	pushWorkspaceGitTags
} from '#lib/client/cometmind.js';

vi.mock('#lib/client/cometmind.js', async (importOriginal) => {
	const actual = await importOriginal<typeof import('#lib/client/cometmind.js')>();
	return {
		...actual,
	fetchWorkspaceGit: vi.fn(),
	listWorkspaceGitRemotes: vi.fn(),
	pushWorkspaceGit: vi.fn(),
	pushWorkspaceGitTags: vi.fn(),
	getWorkspaceGitGraph: vi.fn(),
	getWorkspaceGitStatus: vi.fn()
	};
});

const graphPayload = {
	is_repo: true,
	head_sha: 'abc123',
	head_branch: 'main',
	commits: [],
	truncated: false
};

describe('createGitGraphViewController refresh', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(getWorkspaceGitGraph).mockResolvedValue(graphPayload);
		vi.mocked(getWorkspaceGitStatus).mockResolvedValue({
			is_repo: true,
			branch: 'main',
			files: []
		});
		vi.mocked(fetchWorkspaceGit).mockResolvedValue({ ok: true });
		vi.mocked(listWorkspaceGitRemotes).mockResolvedValue(['origin']);
		vi.mocked(pushWorkspaceGit).mockResolvedValue({ ok: true });
		vi.mocked(pushWorkspaceGitTags).mockResolvedValue({ ok: true });
	});

	it('fetches remotes then reloads the graph', async () => {
		const ctrl = createGitGraphViewController({
			getWorkspacePath: () => '/tmp/project'
		});
		await ctrl.refresh();
		expect(fetchWorkspaceGit).toHaveBeenCalledWith('/tmp/project');
		expect(getWorkspaceGitGraph).toHaveBeenCalled();
		expect(ctrl.graph).toEqual(graphPayload);
	});

	it('surfaces fetch errors without blocking graph reload', async () => {
		vi.mocked(fetchWorkspaceGit).mockRejectedValue(new Error('offline'));
		const ctrl = createGitGraphViewController({
			getWorkspacePath: () => '/tmp/project'
		});
		await ctrl.refresh();
		expect(ctrl.actionError).toContain('offline');
		expect(getWorkspaceGitGraph).toHaveBeenCalled();
		expect(ctrl.graph).toEqual(graphPayload);
	});

	it('pushes the current branch then refreshes remotes', async () => {
		const ctrl = createGitGraphViewController({
			getWorkspacePath: () => '/tmp/project'
		});
		await ctrl.beginPush('push-branch');
		expect(pushWorkspaceGit).toHaveBeenCalledWith('/tmp/project', { remote: 'origin' });
		expect(fetchWorkspaceGit).toHaveBeenCalled();
	});

	it('surfaces push errors without refreshing when push fails', async () => {
		vi.mocked(pushWorkspaceGit).mockRejectedValue(new Error('push rejected'));
		const ctrl = createGitGraphViewController({
			getWorkspacePath: () => '/tmp/project'
		});
		await ctrl.beginPush('push-branch');
		expect(ctrl.actionError).toContain('push rejected');
	});

	it('ignores overlapping refresh clicks while loading', async () => {
		let resolveFetch: (() => void) | undefined;
		vi.mocked(fetchWorkspaceGit).mockImplementation(
			() =>
				new Promise((resolve) => {
					resolveFetch = () => resolve({ ok: true });
				})
		);
		const ctrl = createGitGraphViewController({
			getWorkspacePath: () => '/tmp/project'
		});
		const first = ctrl.refresh();
		expect(ctrl.loading).toBe(true);
		const second = ctrl.refresh();
		resolveFetch?.();
		await Promise.all([first, second]);
		expect(fetchWorkspaceGit).toHaveBeenCalledTimes(1);
	});
});
