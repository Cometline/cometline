export type FileTreeNode = {
	name: string;
	/** Present on file leaves; workspace- or wiki-relative path with `/` separators. */
	path?: string;
	children?: FileTreeNode[];
};

function compareNodes(a: FileTreeNode, b: FileTreeNode): number {
	const aDir = Boolean(a.children);
	const bDir = Boolean(b.children);
	if (aDir !== bDir) return aDir ? -1 : 1;
	return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
}

function sortTree(nodes: FileTreeNode[]): void {
	nodes.sort(compareNodes);
	for (const node of nodes) {
		if (node.children) sortTree(node.children);
	}
}

/**
 * Builds a nested directory tree from flat relative file paths.
 * Directory-only segments become nodes with `children`; files get `path`.
 */
export function buildFileTree(paths: string[]): FileTreeNode[] {
	type MutableNode = {
		name: string;
		path?: string;
		children?: Map<string, MutableNode>;
	};

	const root = new Map<string, MutableNode>();

	for (const raw of paths) {
		const normalized = raw.trim().replace(/\\/g, '/').replace(/^\/+/, '');
		if (!normalized) continue;
		const parts = normalized.split('/').filter(Boolean);
		if (parts.length === 0) continue;

		let current = root;
		for (let i = 0; i < parts.length; i++) {
			const part = parts[i]!;
			const isLeaf = i === parts.length - 1;
			let node = current.get(part);
			if (!node) {
				node = { name: part };
				current.set(part, node);
			}
			if (isLeaf) {
				node.path = normalized;
				continue;
			}
			if (!node.children) node.children = new Map();
			current = node.children;
		}
	}

	function toNodes(map: Map<string, MutableNode>): FileTreeNode[] {
		const nodes: FileTreeNode[] = [];
		for (const node of map.values()) {
			if (node.children) {
				nodes.push({
					name: node.name,
					...(node.path ? { path: node.path } : {}),
					children: toNodes(node.children)
				});
			} else {
				nodes.push({ name: node.name, path: node.path });
			}
		}
		return nodes;
	}

	const tree = toNodes(root);
	sortTree(tree);
	return tree;
}

export function fileTreeDirKey(parentKey: string, name: string): string {
	return parentKey ? `${parentKey}/${name}` : name;
}

/** Flat list of currently visible tree rows (respecting expansion). */
export type FileTreeVisibleRow =
	| { kind: 'dir'; key: string; name: string }
	| { kind: 'file'; key: string; name: string; path: string };

export function flattenVisibleFileTreeRows(
	nodes: FileTreeNode[],
	expanded: Record<string, boolean>,
	parentKey = ''
): FileTreeVisibleRow[] {
	const rows: FileTreeVisibleRow[] = [];
	for (const node of nodes) {
		const key = fileTreeDirKey(parentKey, node.name);
		const hasChildren = Boolean(node.children?.length);
		if (hasChildren) {
			rows.push({ kind: 'dir', key, name: node.name });
			if (expanded[key] && node.children) {
				rows.push(...flattenVisibleFileTreeRows(node.children, expanded, key));
			}
			continue;
		}
		if (node.path) {
			rows.push({ kind: 'file', key, name: node.name, path: node.path });
		}
	}
	return rows;
}

/** Ancestor directory keys that must be expanded to reveal the given file paths. */
export function dirKeysToExpandForPaths(paths: readonly string[]): Record<string, boolean> {
	const expanded: Record<string, boolean> = {};
	for (const raw of paths) {
		const normalized = raw.trim().replace(/\\/g, '/').replace(/^\/+/, '');
		if (!normalized) continue;
		const parts = normalized.split('/').filter(Boolean);
		if (parts.length < 2) continue;
		let key = '';
		for (let i = 0; i < parts.length - 1; i++) {
			key = fileTreeDirKey(key, parts[i]!);
			expanded[key] = true;
		}
	}
	return expanded;
}
