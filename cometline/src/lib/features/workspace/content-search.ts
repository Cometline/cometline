import {
	searchWikiContent,
	searchWorkspaceContent,
	type WorkspaceContentMatch
} from '#lib/client/cometmind.js';

export async function runContentSearch(
	source: 'workspace' | 'wiki',
	query: string,
	workspacePath: string
): Promise<WorkspaceContentMatch[]> {
	const term = query.trim();
	if (!term || (source === 'workspace' && !workspacePath)) return [];
	const result =
		source === 'wiki'
			? await searchWikiContent(term)
			: await searchWorkspaceContent(workspacePath, term);
	return result.matches;
}
