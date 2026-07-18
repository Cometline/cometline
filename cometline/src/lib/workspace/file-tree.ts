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
