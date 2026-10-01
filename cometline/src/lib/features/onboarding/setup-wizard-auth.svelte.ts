// Codex/xAI browser-session auth state (mirrors SettingsProvidersPanel).
type CodexAuthStatus = {
	authenticated: boolean;
	authPath: string;
	accountID?: string;
	error?: string;
};

type XaiAuthStatus = {
	authenticated: boolean;
	authPath: string;
	error?: string;
};

export function createSetupWizardAuth() {
	let codexAuthStatus = $state<CodexAuthStatus | undefined>();
	let checkingCodexAuth = $state(false);
	let startingCodexLogin = $state(false);
	let xaiAuthStatus = $state<XaiAuthStatus | undefined>();
	let checkingXaiAuth = $state(false);
	let startingXaiLogin = $state(false);

	async function refreshCodexAuthStatus() {
		if (!window.electronAPI?.getCodexAuthStatus || checkingCodexAuth) return;
		checkingCodexAuth = true;
		try {
			codexAuthStatus = await window.electronAPI.getCodexAuthStatus();
		} finally {
			checkingCodexAuth = false;
		}
	}

	async function startCodexLogin() {
		if (!window.electronAPI?.startCodexLogin || startingCodexLogin) return;
		startingCodexLogin = true;
		try {
			await window.electronAPI.startCodexLogin();
			// Give the browser a moment, then refresh status.
			setTimeout(() => void refreshCodexAuthStatus(), 1500);
		} catch (err) {
			codexAuthStatus = {
				authenticated: false,
				authPath: '',
				error: err instanceof Error ? err.message : 'Failed to start Codex login.'
			};
		} finally {
			startingCodexLogin = false;
		}
	}

	async function refreshXaiAuthStatus() {
		if (!window.electronAPI?.getXaiAuthStatus || checkingXaiAuth) return;
		checkingXaiAuth = true;
		try {
			xaiAuthStatus = await window.electronAPI.getXaiAuthStatus();
		} finally {
			checkingXaiAuth = false;
		}
	}

	async function startXaiLogin() {
		if (!window.electronAPI?.startXaiLogin || startingXaiLogin) return;
		startingXaiLogin = true;
		try {
			await window.electronAPI.startXaiLogin();
			setTimeout(() => void refreshXaiAuthStatus(), 1500);
		} catch (err) {
			xaiAuthStatus = {
				authenticated: false,
				authPath: '',
				error: err instanceof Error ? err.message : 'Failed to start Grok login.'
			};
		} finally {
			startingXaiLogin = false;
		}
	}

	return {
		get codexAuthStatus() {
			return codexAuthStatus;
		},
		get checkingCodexAuth() {
			return checkingCodexAuth;
		},
		get startingCodexLogin() {
			return startingCodexLogin;
		},
		get xaiAuthStatus() {
			return xaiAuthStatus;
		},
		get checkingXaiAuth() {
			return checkingXaiAuth;
		},
		get startingXaiLogin() {
			return startingXaiLogin;
		},
		refreshCodexAuthStatus,
		startCodexLogin,
		refreshXaiAuthStatus,
		startXaiLogin
	};
}

export type SetupWizardAuth = ReturnType<typeof createSetupWizardAuth>;
