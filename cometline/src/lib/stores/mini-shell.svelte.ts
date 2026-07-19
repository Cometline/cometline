function createMiniShellStore() {
	let sidebarOpen = $state(false);

	return {
		get sidebarOpen() {
			return sidebarOpen;
		},
		toggleSidebar() {
			sidebarOpen = !sidebarOpen;
		},
		openSidebar() {
			sidebarOpen = true;
		},
		closeSidebar() {
			sidebarOpen = false;
		}
	};
}

export const miniShellStore = createMiniShellStore();
