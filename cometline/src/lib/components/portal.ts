/** Move a node to `document.body` so fixed UI escapes ancestor stacking/overflow. */
export function portal(node: HTMLElement) {
	document.body.appendChild(node);
	return {
		destroy() {
			node.remove();
		}
	};
}
