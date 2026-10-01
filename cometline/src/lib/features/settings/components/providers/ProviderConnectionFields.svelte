<script lang="ts">
	import { LogIn, LoaderCircle, RefreshCw } from '@lucide/svelte';
	import type { ProviderConfig, ProviderMethod } from '$lib/types';
	import { isFixedBuiltinProvider } from '$lib/features/settings/schema';

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

	let {
		provider,
		codexAuthStatus,
		checkingCodexAuth = false,
		startingCodexLogin = false,
		xaiAuthStatus,
		checkingXaiAuth = false,
		startingXaiLogin = false,
		onUpdate,
		onSetMethod,
		onStartCodexLogin,
		onRefreshCodexAuth,
		onStartXaiLogin,
		onRefreshXaiAuth
	}: {
		provider: ProviderConfig;
		codexAuthStatus?: CodexAuthStatus;
		checkingCodexAuth?: boolean;
		startingCodexLogin?: boolean;
		xaiAuthStatus?: XaiAuthStatus;
		checkingXaiAuth?: boolean;
		startingXaiLogin?: boolean;
		onUpdate: (patch: Partial<ProviderConfig>) => void;
		onSetMethod: (method: ProviderMethod) => void;
		onStartCodexLogin: () => void;
		onRefreshCodexAuth: () => void;
		onStartXaiLogin: () => void;
		onRefreshXaiAuth: () => void;
	} = $props();

	function methodNeedsApiKey(method: ProviderMethod) {
		return method !== 'codex' && method !== 'xai' && method !== 'ollama';
	}
</script>

<div class="form-grid">
	<label class:locked={isFixedBuiltinProvider(provider.id)}>
		<span>Name</span>
		<input
			class:locked={isFixedBuiltinProvider(provider.id)}
			value={provider.name}
			oninput={(e) => onUpdate({ name: e.currentTarget.value })}
			placeholder="Provider name"
			spellcheck="false"
			disabled={isFixedBuiltinProvider(provider.id)}
		/>
	</label>

	<label class:locked={isFixedBuiltinProvider(provider.id)}>
		<span>Method</span>
		<select
			class:locked={isFixedBuiltinProvider(provider.id)}
			value={provider.method}
			onchange={(e) => onSetMethod(e.currentTarget.value as ProviderMethod)}
			disabled={isFixedBuiltinProvider(provider.id)}
		>
			<option value="codex">ChatGPT Codex</option>
			<option value="xai">xAI Grok Subscription</option>
			<option value="openai">OpenAI</option>
			<option value="anthropic">Anthropic</option>
			<option value="ollama">Ollama Local</option>
			<option value="opencode-go">OpenCode Go</option>
			<option value="openai-compatible">Advanced / Custom endpoint</option>
		</select>
	</label>

	{#if provider.method !== 'ollama'}
		<label>
			<span>Base URL</span>
			<input
				value={provider.baseURL}
				oninput={(e) => onUpdate({ baseURL: e.currentTarget.value })}
				placeholder="https://example.com/v1"
				spellcheck="false"
			/>
		</label>
	{/if}

	{#if methodNeedsApiKey(provider.method)}
		<label>
			<span>API Key</span>
			<input
				value={provider.apiKey}
				oninput={(e) => onUpdate({ apiKey: e.currentTarget.value })}
				type="password"
				placeholder="sk-..."
				spellcheck="false"
			/>
		</label>
	{:else if provider.method === 'codex'}
		<div class="field-note">
			<span>Authentication</span>
			<p>
				Uses your ChatGPT Plus/Pro browser sign-in and stores a local Codex-compatible
				session at <code>~/.codex/auth.json</code>. No API key or Codex CLI install is
				required.
			</p>
			{#if codexAuthStatus}
				<p class:ok={codexAuthStatus.authenticated}>
					{codexAuthStatus.authenticated
						? 'Signed in with ChatGPT browser session.'
						: (codexAuthStatus.error ?? 'Not signed in.')}
				</p>
			{/if}
			<div class="inline-actions">
				<button
					class="secondary"
					type="button"
					onclick={onStartCodexLogin}
					disabled={startingCodexLogin || !window.electronAPI?.startCodexLogin}
				>
					{#if startingCodexLogin}<span class="spin"><LoaderCircle size={14} /></span
						>{:else}<LogIn size={14} />{/if}
					Sign in with ChatGPT
				</button>
				<button
					class="secondary"
					type="button"
					onclick={onRefreshCodexAuth}
					disabled={checkingCodexAuth || !window.electronAPI?.getCodexAuthStatus}
				>
					{#if checkingCodexAuth}<span class="spin"><LoaderCircle size={14} /></span
						>{:else}<RefreshCw size={14} />{/if}
					Check session
				</button>
			</div>
		</div>
	{:else if provider.method === 'xai'}
		<div class="field-note">
			<span>Authentication</span>
			<p>
				Uses your SuperGrok or X Premium subscription through xAI OAuth. The session is
				stored locally at <code>~/.cometmind/xai/auth.json</code>.
			</p>
			{#if xaiAuthStatus}
				<p class:ok={xaiAuthStatus.authenticated}>
					{xaiAuthStatus.authenticated
						? 'Signed in with Grok subscription.'
						: (xaiAuthStatus.error ?? 'Not signed in.')}
				</p>
			{/if}
			<div class="inline-actions">
				<button
					class="secondary"
					type="button"
					onclick={onStartXaiLogin}
					disabled={startingXaiLogin || !window.electronAPI?.startXaiLogin}
				>
					{#if startingXaiLogin}<span class="spin"><LoaderCircle size={14} /></span
						>{:else}<LogIn size={14} />{/if}
					Sign in with Grok
				</button>
				<button
					class="secondary"
					type="button"
					onclick={onRefreshXaiAuth}
					disabled={checkingXaiAuth || !window.electronAPI?.getXaiAuthStatus}
				>
					{#if checkingXaiAuth}<span class="spin"><LoaderCircle size={14} /></span
						>{:else}<RefreshCw size={14} />{/if}
					Check session
				</button>
			</div>
		</div>
	{/if}
</div>

<style>
	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
		margin-bottom: 16px;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.field-note {
		display: grid;
		grid-column: 1 / -1;
		gap: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.55);
		padding: 10px 11px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.field-note span {
		font-weight: 700;
	}

	.field-note p {
		max-width: 640px;
		font-weight: 500;
		line-height: 1.45;
		margin: 0;
	}

	.field-note p.ok {
		color: var(--color-24745d);
		font-weight: 650;
	}

	.inline-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		padding-top: 2px;
	}
</style>
