import {
	loadWorkspacePanelFileOptions
} from '$lib/workspace/workspace-panel-input-options';
import { normalizeWorkspacePath } from '$lib/workspace/file-index';

const DEFAULT_LIMIT = 8;

export async function loadPanelFileOptions(
	workspacePath: string,
	query: string,
	limit = DEFAULT_LIMIT
): Promise<string[]> {
	const trimmed = query.trim();
	if (!trimmed) return [];

	const normalizedWorkspace = normalizeWorkspacePath(workspacePath);
	if (!normalizedWorkspace || normalizedWorkspace === '/') return [];

	return loadWorkspacePanelFileOptions(normalizedWorkspace, trimmed, limit);
}
