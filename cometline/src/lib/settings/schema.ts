import { z } from 'zod';
import {
	DEFAULT_HERO_COMPOSER_APPEARANCE,
	normalizeHeroComposerAppearance
} from '../hero-composer-appearance';
import { defaultKeyboardShortcuts, normalizeKeyboardShortcuts } from '../keyboard-shortcuts';
import type {
	AppSettings,
	CustomPersona,
	AppearanceSettings,
	CaretTrailSettings,
	ProviderConfig,
	ProviderMethod,
	ProviderSettings
} from '../types';
import {
	DEFAULT_CONTEXT_WINDOW_LIMIT,
	normalizeContextWindowLimit,
	type ContextWindowLimit
} from '../context-window';
import {
	migratePersonaIdFromIconVariant,
	normalizeCustomPersonas as normalizeCustomPersonaList,
	normalizePersonaId as resolveNormalizedPersonaId
} from '../personas';

export const VALID_PROVIDER_METHODS: ProviderMethod[] = [
	'openai-compatible',
	'openai',
	'anthropic',
	'opencode-go',
	'codex'
];

const BUILTIN_PROVIDER_NAMES: Record<string, string> = {
	'openai-compatible': 'OpenAI Compatible',
	anthropic: 'Anthropic',
	openai: 'OpenAI',
	'opencode-go': 'OpenCode Go',
	codex: 'ChatGPT Codex'
};

function providerNameOrDefault(
	provider: Partial<ProviderConfig>,
	fallback: ProviderConfig | undefined,
	id: string
) {
	const name = String(provider.name ?? '').trim();
	if (name) return name;
	const fallbackName = String(fallback?.name ?? '').trim();
	if (fallbackName) return fallbackName;
	return BUILTIN_PROVIDER_NAMES[id] ?? 'Provider';
}

export interface CometMindACPSettings {
	command: string;
	args: string[];
	timeout: string;
}

export interface CometMindDiscordGatewaySettings {
	enabled: boolean;
	botToken: string;
	botTokenEnv: string;
	providerId: string;
	modelId: string;
	allowedUsers: string[];
	allowedChannels: string[];
	requireMention: boolean;
	workspacePath: string;
}

export interface CometMindSkillsSettings {
	enabled: boolean;
	roots: string[];
	includeOpenCode: boolean;
	includeClaude: boolean;
	mirrorToCometMind: boolean;
}

export interface CometMindMemorySettings {
	extractionProviderId: string;
	extractionModel: string;
	embedding: {
		providerId: string;
		provider: string;
		model: string;
		baseURL: string;
		apiKey: string;
	};
}

export interface CometMindStorageSettings {
	cleanupIntervalMinutes: number;
	retentionDays: number;
	maxSessionsPerWorkspace: number;
	archivedMemoryPurgeDays: number;
	deletedJobPurgeDays: number;
	vacuumAfterPurge: boolean;
}

export type MCPTransport = 'stdio' | 'http' | 'sse';

export interface MCPOAuthSettings {
	clientId?: string;
	scopes?: string[];
	authorizationUrl?: string;
	tokenUrl?: string;
}

export interface MCPServerConfig {
	id: string;
	name: string;
	enabled: boolean;
	transport: MCPTransport;
	command?: string;
	args?: string[];
	env?: Record<string, string>;
	url?: string;
	headers?: Record<string, string>;
	oauth?: MCPOAuthSettings;
	allowedTools?: string[];
}

export interface CometMindMCPSettings {
	enabled: boolean;
	servers: MCPServerConfig[];
}

export interface CometMindJobsNotificationSettings {
	enabled: boolean;
	onClaimed: boolean;
	onCompleted: boolean;
	onReleased: boolean;
}

export interface CometMindJobsSettings {
	notifications: CometMindJobsNotificationSettings;
	leaseMinutes: number;
	deletedPurgeDays: number;
	reconcileIntervalSeconds: number;
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

const LOG_LEVELS: LogLevel[] = ['debug', 'info', 'warn', 'error'];

function normalizeLogLevel(value: unknown): LogLevel {
	const raw = String(value ?? '')
		.trim()
		.toLowerCase();
	if (LOG_LEVELS.includes(raw as LogLevel)) {
		return raw as LogLevel;
	}
	return 'error';
}

export interface CometMindSettings {
	systemPromptPath: string;
	maxTokens: number;
	logLevel: LogLevel;
	contextWindowLimit: ContextWindowLimit;
	titleProviderId: string;
	titleModelId: string;
	acp: CometMindACPSettings;
	skills: CometMindSkillsSettings;
	memory: CometMindMemorySettings;
	storage: CometMindStorageSettings;
	gateway: {
		discord: CometMindDiscordGatewaySettings;
	};
	mcp: CometMindMCPSettings;
	jobs: CometMindJobsSettings;
}

export interface RuntimeProviderEntry {
	id: string;
	name: string;
	method: string;
	baseURL: string;
	apiKey: string;
	model: string;
}

export interface RuntimeSettingsSlice {
	provider: string;
	model: string;
	baseURL: string;
	maxTokens: number;
	maxSteps: number;
	systemPromptPath: string;
	providers: RuntimeProviderEntry[];
	acp: CometMindACPSettings;
	skills: CometMindSkillsSettings;
	memory: CometMindMemorySettings;
	gateway: CometMindSettings['gateway'];
	mcp: CometMindMCPSettings;
}

const DEFAULT_PROVIDERS: ProviderConfig[] = [
	{
		id: 'codex',
		name: 'ChatGPT Codex',
		method: 'codex',
		enabled: false,
		baseURL: 'https://chatgpt.com/backend-api/codex',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'openai',
		name: 'OpenAI',
		method: 'openai',
		enabled: false,
		baseURL: 'https://api.openai.com/v1',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'anthropic',
		name: 'Anthropic',
		method: 'anthropic',
		enabled: false,
		baseURL: 'https://api.anthropic.com',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'opencode-go',
		name: 'OpenCode Go',
		method: 'opencode-go',
		enabled: false,
		baseURL: 'https://opencode.ai/zen/go/v1',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'openai-compatible',
		name: 'OpenAI Compatible',
		method: 'openai-compatible',
		enabled: false,
		baseURL: '',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	}
];

function looksLikeDiscordBotToken(value: string): boolean {
	const parts = value.split('.');
	if (parts.length !== 3) return false;
	return parts[0].length >= 18 && parts[1].length >= 4 && parts[2].length >= 20;
}

function migrateDiscordTokenFields(discord: Partial<CometMindDiscordGatewaySettings>) {
	const defaults = defaultCometMindSettings().gateway.discord;
	let botToken = String(discord.botToken ?? '').trim();
	let botTokenEnv =
		String(discord.botTokenEnv ?? defaults.botTokenEnv).trim() || defaults.botTokenEnv;
	if (!botToken && looksLikeDiscordBotToken(botTokenEnv)) {
		botToken = botTokenEnv;
		botTokenEnv = defaults.botTokenEnv;
	}
	return { botToken, botTokenEnv };
}

function cleanStringList(values: unknown): string[] {
	if (!Array.isArray(values)) return [];
	return values.map((v) => String(v).trim()).filter(Boolean);
}

function normalizeNonNegativeInt(value: unknown, fallback: number): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) return fallback;
	return Math.max(0, Math.floor(value));
}

function normalizePositiveInt(value: unknown, fallback: number): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) return fallback;
	return Math.max(1, Math.floor(value));
}

function cleanStringMap(values: unknown): Record<string, string> {
	if (!values || typeof values !== 'object') return {};
	const out: Record<string, string> = {};
	for (const [key, value] of Object.entries(values as Record<string, unknown>)) {
		const k = String(key).trim();
		const v = String(value ?? '').trim();
		if (k && v) out[k] = v;
	}
	return out;
}

function slugifyMCPId(name: string, existing: Set<string>): string {
	const base =
		name
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '') || 'server';
	let candidate = base;
	let n = 2;
	while (existing.has(candidate)) {
		candidate = `${base}-${n++}`;
	}
	existing.add(candidate);
	return candidate;
}

const VALID_MCP_TRANSPORTS: MCPTransport[] = ['stdio', 'http', 'sse'];

function normalizeMCPTransport(value: unknown, fallback: MCPTransport): MCPTransport {
	const raw = String(value ?? '').trim() as MCPTransport;
	return VALID_MCP_TRANSPORTS.includes(raw) ? raw : fallback;
}

function normalizeMCPOAuth(
	input: Partial<MCPOAuthSettings> | undefined
): MCPOAuthSettings | undefined {
	if (!input) return undefined;
	const clientId = String(input.clientId ?? '').trim();
	const authorizationUrl = String(input.authorizationUrl ?? '').trim();
	const tokenUrl = String(input.tokenUrl ?? '').trim();
	const scopes = cleanStringList(input.scopes);
	if (!clientId && !authorizationUrl && !tokenUrl && scopes.length === 0) {
		return undefined;
	}
	return {
		clientId,
		scopes,
		authorizationUrl,
		tokenUrl
	};
}

export function defaultCometMindMCPSettings(): CometMindMCPSettings {
	return { enabled: false, servers: [] };
}

function normalizeMCPServer(
	input: Partial<MCPServerConfig> | undefined,
	existingIds: Set<string>
): MCPServerConfig | null {
	if (!input) return null;
	const name = String(input.name ?? '').trim();
	const id = String(input.id ?? '').trim() || (name ? slugifyMCPId(name, existingIds) : '');
	if (!id) return null;
	if (!existingIds.has(id)) existingIds.add(id);
	const transport = normalizeMCPTransport(input.transport, 'stdio');
	return {
		id,
		name: name || id,
		enabled: typeof input.enabled === 'boolean' ? input.enabled : true,
		transport,
		command: String(input.command ?? '').trim(),
		args: cleanStringList(input.args),
		env: cleanStringMap(input.env),
		url: String(input.url ?? '').trim(),
		headers: cleanStringMap(input.headers),
		oauth: normalizeMCPOAuth(input.oauth),
		allowedTools: cleanStringList(input.allowedTools)
	};
}

function normalizeCometMindMCPSettings(
	input: Partial<CometMindMCPSettings> | undefined
): CometMindMCPSettings {
	const defaults = defaultCometMindMCPSettings();
	const enabled = typeof input?.enabled === 'boolean' ? input.enabled : defaults.enabled;
	const ids = new Set<string>();
	const servers: MCPServerConfig[] = [];
	if (Array.isArray(input?.servers)) {
		for (const srv of input.servers) {
			const normalized = normalizeMCPServer(srv, ids);
			if (normalized) servers.push(normalized);
		}
	}
	return { enabled, servers };
}

export function defaultCometMindJobsSettings(): CometMindJobsSettings {
	return {
		notifications: {
			enabled: true,
			onClaimed: true,
			onCompleted: true,
			onReleased: false
		},
		leaseMinutes: 30,
		deletedPurgeDays: 30,
		reconcileIntervalSeconds: 120
	};
}

export function defaultCometMindStorageSettings(): CometMindStorageSettings {
	return {
		cleanupIntervalMinutes: 60,
		retentionDays: 90,
		maxSessionsPerWorkspace: 0,
		archivedMemoryPurgeDays: 90,
		deletedJobPurgeDays: 30,
		vacuumAfterPurge: true
	};
}

export function defaultCometMindSettings(workspacePath = ''): CometMindSettings {
	return {
		systemPromptPath: '',
		maxTokens: 2048,
		logLevel: 'error',
		contextWindowLimit: DEFAULT_CONTEXT_WINDOW_LIMIT,
		titleProviderId: '',
		titleModelId: '',
		acp: {
			command: 'opencode',
			args: ['acp'],
			timeout: '30m'
		},
		skills: {
			enabled: true,
			roots: [],
			includeOpenCode: true,
			includeClaude: true,
			mirrorToCometMind: true
		},
		memory: {
			extractionProviderId: '',
			extractionModel: '',
			embedding: {
				providerId: '',
				provider: '',
				model: '',
				baseURL: '',
				apiKey: ''
			}
		},
		storage: defaultCometMindStorageSettings(),
		gateway: {
			discord: {
				enabled: false,
				botToken: '',
				botTokenEnv: 'DISCORD_BOT_TOKEN',
				providerId: '',
				modelId: '',
				allowedUsers: [],
				allowedChannels: [],
				requireMention: true,
				workspacePath
			}
		},
		mcp: defaultCometMindMCPSettings(),
		jobs: defaultCometMindJobsSettings()
	};
}

export function normalizeCometMindSettings(
	input: Partial<CometMindSettings> | undefined,
	fallbackWorkspacePath = ''
): CometMindSettings {
	const defaults = defaultCometMindSettings(fallbackWorkspacePath);
	const acp: Partial<CometMindACPSettings> = input?.acp ?? {};
	const skills: Partial<CometMindSkillsSettings> = input?.skills ?? {};
	const memory: Partial<CometMindMemorySettings> = input?.memory ?? {};
	const embedding: Partial<CometMindMemorySettings['embedding']> = memory.embedding ?? {};
	const storage: Partial<CometMindStorageSettings> = input?.storage ?? {};
	const discord: Partial<CometMindDiscordGatewaySettings> = input?.gateway?.discord ?? {};
	const mcp = normalizeCometMindMCPSettings(input?.mcp);
	const jobsInput: Partial<CometMindJobsSettings> = input?.jobs ?? {};
	const jobsDefaults = defaults.jobs;
	const jobsNotifications: Partial<CometMindJobsNotificationSettings> =
		jobsInput.notifications ?? {};
	const args = Array.isArray(acp.args)
		? acp.args.map((a) => String(a).trim()).filter(Boolean)
		: defaults.acp.args;
	const { botToken, botTokenEnv } = migrateDiscordTokenFields(discord);

	return {
		systemPromptPath: String(input?.systemPromptPath ?? defaults.systemPromptPath).trim(),
		maxTokens: normalizePositiveInt(input?.maxTokens, defaults.maxTokens),
		logLevel: normalizeLogLevel(input?.logLevel ?? defaults.logLevel),
		contextWindowLimit: normalizeContextWindowLimit(
			input?.contextWindowLimit ?? defaults.contextWindowLimit
		),
		titleProviderId: String(input?.titleProviderId ?? defaults.titleProviderId).trim(),
		titleModelId: String(input?.titleModelId ?? defaults.titleModelId).trim(),
		acp: {
			command: String(acp.command ?? defaults.acp.command).trim() || defaults.acp.command,
			args: args.length > 0 ? args : defaults.acp.args,
			timeout: String(acp.timeout ?? defaults.acp.timeout).trim() || defaults.acp.timeout
		},
		skills: {
			enabled: typeof skills.enabled === 'boolean' ? skills.enabled : defaults.skills.enabled,
			roots: [],
			includeOpenCode: true,
			includeClaude: true,
			mirrorToCometMind: true
		},
		memory: {
			extractionProviderId: String(
				memory.extractionProviderId ?? defaults.memory.extractionProviderId
			).trim(),
			extractionModel: String(
				memory.extractionModel ?? defaults.memory.extractionModel
			).trim(),
			embedding: {
				providerId: String(
					embedding.providerId ?? defaults.memory.embedding.providerId
				).trim(),
				provider: String(embedding.provider ?? defaults.memory.embedding.provider).trim(),
				model: String(embedding.model ?? defaults.memory.embedding.model).trim(),
				baseURL: String(embedding.baseURL ?? defaults.memory.embedding.baseURL).trim(),
				apiKey: String(embedding.apiKey ?? defaults.memory.embedding.apiKey).trim()
			}
		},
		storage: {
			cleanupIntervalMinutes: normalizeNonNegativeInt(
				storage.cleanupIntervalMinutes,
				defaults.storage.cleanupIntervalMinutes
			),
			retentionDays: normalizeNonNegativeInt(
				storage.retentionDays,
				defaults.storage.retentionDays
			),
			maxSessionsPerWorkspace: normalizeNonNegativeInt(
				storage.maxSessionsPerWorkspace,
				defaults.storage.maxSessionsPerWorkspace
			),
			archivedMemoryPurgeDays: normalizeNonNegativeInt(
				storage.archivedMemoryPurgeDays,
				defaults.storage.archivedMemoryPurgeDays
			),
			deletedJobPurgeDays: normalizeNonNegativeInt(
				storage.deletedJobPurgeDays,
				defaults.storage.deletedJobPurgeDays
			),
			vacuumAfterPurge:
				typeof storage.vacuumAfterPurge === 'boolean'
					? storage.vacuumAfterPurge
					: defaults.storage.vacuumAfterPurge
		},
		gateway: {
			discord: {
				enabled:
					typeof discord.enabled === 'boolean'
						? discord.enabled
						: defaults.gateway.discord.enabled,
				botToken,
				botTokenEnv,
				providerId: String(
					discord.providerId ?? defaults.gateway.discord.providerId
				).trim(),
				modelId: String(discord.modelId ?? defaults.gateway.discord.modelId).trim(),
				allowedUsers: cleanStringList(discord.allowedUsers),
				allowedChannels: cleanStringList(discord.allowedChannels),
				requireMention:
					typeof discord.requireMention === 'boolean'
						? discord.requireMention
						: defaults.gateway.discord.requireMention,
				workspacePath:
					String(
						discord.workspacePath ?? defaults.gateway.discord.workspacePath
					).trim() || defaults.gateway.discord.workspacePath
			}
		},
		mcp,
		jobs: {
			notifications: {
				enabled:
					typeof jobsNotifications.enabled === 'boolean'
						? jobsNotifications.enabled
						: jobsDefaults.notifications.enabled,
				onClaimed:
					typeof jobsNotifications.onClaimed === 'boolean'
						? jobsNotifications.onClaimed
						: jobsDefaults.notifications.onClaimed,
				onCompleted:
					typeof jobsNotifications.onCompleted === 'boolean'
						? jobsNotifications.onCompleted
						: jobsDefaults.notifications.onCompleted,
				onReleased:
					typeof jobsNotifications.onReleased === 'boolean'
						? jobsNotifications.onReleased
						: jobsDefaults.notifications.onReleased
			},
			leaseMinutes: normalizePositiveInt(jobsInput.leaseMinutes, jobsDefaults.leaseMinutes),
			deletedPurgeDays: normalizeNonNegativeInt(
				jobsInput.deletedPurgeDays,
				jobsDefaults.deletedPurgeDays
			),
			reconcileIntervalSeconds: normalizePositiveInt(
				jobsInput.reconcileIntervalSeconds,
				jobsDefaults.reconcileIntervalSeconds
			)
		}
	};
}

export function cloneCometMindSettings(settings: CometMindSettings): CometMindSettings {
	return {
		systemPromptPath: settings.systemPromptPath,
		maxTokens: settings.maxTokens,
		logLevel: settings.logLevel,
		contextWindowLimit: settings.contextWindowLimit,
		titleProviderId: settings.titleProviderId,
		titleModelId: settings.titleModelId,
		acp: {
			command: settings.acp.command,
			args: [...settings.acp.args],
			timeout: settings.acp.timeout
		},
		skills: {
			...settings.skills,
			roots: [...settings.skills.roots]
		},
		memory: {
			extractionProviderId: settings.memory.extractionProviderId,
			extractionModel: settings.memory.extractionModel,
			embedding: { ...settings.memory.embedding }
		},
		storage: { ...settings.storage },
		gateway: {
			discord: {
				...settings.gateway.discord,
				allowedUsers: [...settings.gateway.discord.allowedUsers],
				allowedChannels: [...settings.gateway.discord.allowedChannels]
			}
		},
		mcp: {
			enabled: settings.mcp.enabled,
			servers: settings.mcp.servers.map((server) => ({
				...server,
				args: [...(server.args ?? [])],
				env: { ...(server.env ?? {}) },
				headers: { ...(server.headers ?? {}) },
				oauth: server.oauth
					? { ...server.oauth, scopes: [...(server.oauth.scopes ?? [])] }
					: undefined,
				allowedTools: [...(server.allowedTools ?? [])]
			}))
		},
		jobs: {
			...settings.jobs,
			notifications: { ...settings.jobs.notifications }
		}
	};
}

function defaultCaretTrailSettings(): CaretTrailSettings {
	return { enabled: true, intensity: 0.72, speed: 0.68 };
}

function normalizeUnit(value: unknown, fallback: number): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) return fallback;
	return Math.min(1, Math.max(0, value));
}

function normalizeCaretTrailSettings(
	settings: Partial<CaretTrailSettings> | undefined
): CaretTrailSettings {
	const defaults = defaultCaretTrailSettings();
	return {
		enabled: typeof settings?.enabled === 'boolean' ? settings.enabled : defaults.enabled,
		intensity: normalizeUnit(settings?.intensity, defaults.intensity),
		speed: normalizeUnit(settings?.speed, defaults.speed)
	};
}

function defaultAppearance(): AppearanceSettings {
	return {
		heroComposer: { ...DEFAULT_HERO_COMPOSER_APPEARANCE },
		caretTrail: defaultCaretTrailSettings()
	};
}

function defaultAppSettings(): AppSettings {
	return {
		openAtLogin: false,
		hasSeenIntro: false,
		hasCompletedSetup: false,
		hasDismissedSetupWizard: false,
		personaId: 'minako',
		personas: { custom: [] },
		miniWindowSessionId: '',
		miniWindowLastActiveAt: 0,
		miniWindowInactivityTimeoutMinutes: 30,
		webPanelWidth: 0
	};
}

function normalizeWebPanelWidth(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) {
		return defaultAppSettings().webPanelWidth;
	}
	return Math.max(0, Math.floor(value));
}

/**
 * Normalizes `app.personaId`, migrating from the legacy `app.iconVariant`
 * field (`'default' | 'man'`) when `personaId` is absent. `customPersonas`
 * must already be normalized so a custom persona id is recognized as valid.
 */
function normalizeAppPersonaId(rawApp: unknown, customPersonas: CustomPersona[]): string {
	const app = (rawApp ?? {}) as { personaId?: unknown; iconVariant?: unknown };
	if (typeof app.personaId === 'string' && app.personaId.trim()) {
		return resolveNormalizedPersonaId(app.personaId, customPersonas);
	}
	if (app.iconVariant !== undefined) {
		return migratePersonaIdFromIconVariant(app.iconVariant);
	}
	return 'minako';
}

function normalizeMiniWindowLastActiveAt(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
	return Math.max(0, Math.floor(value));
}

function normalizeMiniWindowInactivityTimeoutMinutes(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) {
		return defaultAppSettings().miniWindowInactivityTimeoutMinutes;
	}
	return Math.min(24 * 60, Math.max(1, Math.floor(value)));
}

export function cloneProvider(provider: ProviderConfig): ProviderConfig {
	return {
		...provider,
		models: [...provider.models],
		enabledModels: [...provider.enabledModels]
	};
}

export function normalizeProvider(
	provider: Partial<ProviderConfig>,
	fallback?: ProviderConfig
): ProviderConfig {
	const method = VALID_PROVIDER_METHODS.includes(provider.method as ProviderMethod)
		? (provider.method as ProviderMethod)
		: (fallback?.method ?? 'openai-compatible');
	const rawModels = Array.isArray(provider.models) ? provider.models : (fallback?.models ?? []);
	const modelList = rawModels.map((model) => String(model || '').trim()).filter(Boolean);
	const legacySelected = String(provider.selectedModel || fallback?.selectedModel || '').trim();
	const rawEnabledModels = Array.isArray(provider.enabledModels)
		? provider.enabledModels
		: legacySelected
			? [legacySelected]
			: [];
	const enabledModels = rawEnabledModels
		.map((model) => String(model || '').trim())
		.filter((model) => model && modelList.includes(model));
	const id = String(
		provider.id ||
			fallback?.id ||
			`provider-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
	).trim();

	return {
		id,
		name: providerNameOrDefault(provider, fallback, id),
		method,
		enabled:
			typeof provider.enabled === 'boolean' ? provider.enabled : Boolean(fallback?.enabled),
		baseURL: String(provider.baseURL ?? fallback?.baseURL ?? '').trim(),
		apiKey: method === 'codex' ? '' : String(provider.apiKey ?? fallback?.apiKey ?? '').trim(),
		selectedModel: enabledModels[0] || '',
		models: [...modelList],
		enabledModels
	};
}

/** Runtime active provider: first enabled with models, else preferred id, else sidebar order. */
export function resolveActiveProviderId(providers: ProviderConfig[], preferredId?: string): string {
	const preferred = preferredId
		? providers.find((provider) => provider.id === preferredId)
		: undefined;
	if (preferred?.enabled && preferred.enabledModels.length > 0) {
		return preferred.id;
	}
	const enabledWithModels = providers.find(
		(provider) => provider.enabled && provider.enabledModels.length > 0
	);
	if (enabledWithModels) return enabledWithModels.id;
	return providers[0]?.id ?? '';
}

export function normalizeProviders(
	providers: Partial<ProviderConfig>[] | undefined
): ProviderConfig[] {
	const saved = Array.isArray(providers) ? providers : [];
	const normalizedDefaults = DEFAULT_PROVIDERS.map((provider) => {
		const savedProvider = saved.find((p) => p.id === provider.id);
		return normalizeProvider(savedProvider ?? provider, provider);
	});
	const customProviders = saved
		.filter((provider) => !DEFAULT_PROVIDERS.some((p) => p.id === provider.id))
		.map((provider) => normalizeProvider(provider));
	return [...normalizedDefaults, ...customProviders];
}

export function newProvider(id: string): ProviderConfig {
	return {
		id,
		name: 'New Provider',
		method: 'openai-compatible',
		enabled: false,
		baseURL: '',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	};
}

export function migrateSingleProvider(
	saved: Record<string, unknown> | null | undefined
): Partial<ProviderSettings> | null {
	if (!saved || typeof saved !== 'object' || Array.isArray(saved.providers)) return null;
	const id = String(saved.provider || 'openai').trim();
	return {
		providers: [
			{
				id,
				name:
					id === 'opencode-go' ? 'OpenCode Go' : id.charAt(0).toUpperCase() + id.slice(1),
				method:
					id === 'openai' && String(saved.baseURL || '').includes('opencode.ai')
						? 'opencode-go'
						: id === 'openai'
							? 'openai-compatible'
							: (id as ProviderMethod),
				enabled: true,
				baseURL: String(saved.baseURL || '').trim(),
				apiKey: String(saved.apiKey || '').trim(),
				selectedModel: String(saved.selectedModel || '').trim(),
				models: Array.isArray(saved.models)
					? saved.models.map((m) => String(m || '').trim()).filter(Boolean)
					: [],
				enabledModels: saved.selectedModel ? [String(saved.selectedModel).trim()] : []
			}
		],
		activeProviderId: id
	};
}

export function defaultSettings(): ProviderSettings {
	const providers = DEFAULT_PROVIDERS.map(cloneProvider);
	return {
		providers,
		activeProviderId: resolveActiveProviderId(providers),
		defaultModelId: '',
		defaultProviderId: '',
		appearance: defaultAppearance(),
		shortcuts: defaultKeyboardShortcuts(),
		app: defaultAppSettings(),
		cometmind: defaultCometMindSettings()
	};
}

export interface NormalizeSettingsOptions {
	fallbackWorkspacePath?: string;
	systemPromptPath?: string;
}

export function normalizeSettings(
	next: Partial<ProviderSettings>,
	options: NormalizeSettingsOptions = {}
): ProviderSettings {
	const providers = normalizeProviders(next.providers);
	const activeProviderId = resolveActiveProviderId(providers, next.activeProviderId);
	const cometmind = normalizeCometMindSettings(
		next.cometmind,
		options.fallbackWorkspacePath ?? ''
	);
	if (options.systemPromptPath) {
		cometmind.systemPromptPath = options.systemPromptPath;
	}
	const customPersonas = normalizeCustomPersonaList(next.app?.personas?.custom);
	return {
		providers,
		activeProviderId,
		defaultModelId: String(next.defaultModelId ?? '').trim(),
		defaultProviderId: String(next.defaultProviderId ?? '').trim(),
		appearance: {
			heroComposer: normalizeHeroComposerAppearance(next.appearance?.heroComposer),
			caretTrail: normalizeCaretTrailSettings(next.appearance?.caretTrail)
		},
		shortcuts: normalizeKeyboardShortcuts(next.shortcuts),
		app: {
			openAtLogin:
				typeof next.app?.openAtLogin === 'boolean'
					? next.app.openAtLogin
					: defaultAppSettings().openAtLogin,
			hasSeenIntro:
				typeof next.app?.hasSeenIntro === 'boolean'
					? next.app.hasSeenIntro
					: defaultAppSettings().hasSeenIntro,
			hasCompletedSetup:
				typeof next.app?.hasCompletedSetup === 'boolean'
					? next.app.hasCompletedSetup
					: defaultAppSettings().hasCompletedSetup,
			hasDismissedSetupWizard:
				typeof next.app?.hasDismissedSetupWizard === 'boolean'
					? next.app.hasDismissedSetupWizard
					: defaultAppSettings().hasDismissedSetupWizard,
			personaId: normalizeAppPersonaId(next.app, customPersonas),
			personas: { custom: customPersonas },
			miniWindowSessionId: String(next.app?.miniWindowSessionId ?? '').trim(),
			miniWindowLastActiveAt: normalizeMiniWindowLastActiveAt(
				next.app?.miniWindowLastActiveAt
			),
			miniWindowInactivityTimeoutMinutes: normalizeMiniWindowInactivityTimeoutMinutes(
				next.app?.miniWindowInactivityTimeoutMinutes
			),
			webPanelWidth: normalizeWebPanelWidth(next.app?.webPanelWidth)
		},
		cometmind
	};
}

function primaryModel(provider: ProviderConfig): string {
	return provider.enabledModels[0] || provider.selectedModel || provider.models[0] || '';
}

export function runtimeProviders(settings: ProviderSettings): ProviderConfig[] {
	return settings.providers.filter((p) => p.enabled && p.enabledModels.length > 0);
}

export function runtimeSlice(settings: ProviderSettings): RuntimeSettingsSlice | null {
	const providers = runtimeProviders(settings);
	const active =
		providers.find((p) => p.id === settings.activeProviderId) ?? providers[0] ?? null;
	if (!active) return null;

	return {
		provider: active.id,
		model: primaryModel(active),
		baseURL: active.baseURL,
		maxTokens: settings.cometmind.maxTokens,
		maxSteps: 50,
		systemPromptPath: settings.cometmind.systemPromptPath,
		providers: providers.map((p) => ({
			id: p.id,
			name: p.name,
			method: p.method,
			baseURL: p.baseURL,
			apiKey: p.apiKey,
			model: primaryModel(p)
		})),
		acp: { ...settings.cometmind.acp, args: [...settings.cometmind.acp.args] },
		skills: { ...settings.cometmind.skills, roots: [...settings.cometmind.skills.roots] },
		memory: {
			extractionProviderId: settings.cometmind.memory.extractionProviderId,
			extractionModel: settings.cometmind.memory.extractionModel,
			embedding: { ...settings.cometmind.memory.embedding }
		},
		gateway: cloneCometMindSettings(settings.cometmind).gateway,
		mcp: cloneCometMindSettings(settings.cometmind).mcp
	};
}

const providerConfigSchema = z.object({
	id: z.string().min(1),
	name: z.string(),
	method: z.enum(['openai-compatible', 'openai', 'anthropic', 'opencode-go', 'codex']),
	enabled: z.boolean(),
	baseURL: z.string(),
	apiKey: z.string(),
	selectedModel: z.string(),
	models: z.array(z.string()),
	enabledModels: z.array(z.string())
});

const providerSettingsSchema = z.object({
	providers: z.array(providerConfigSchema).min(1),
	activeProviderId: z.string(),
	defaultModelId: z.string(),
	defaultProviderId: z.string(),
	appearance: z.object({
		heroComposer: z.object({
			glowColor: z.string(),
			ringColor: z.string()
		}),
		caretTrail: z.object({
			enabled: z.boolean(),
			intensity: z.number().min(0).max(1),
			speed: z.number().min(0).max(1)
		})
	}),
	shortcuts: z.record(z.string(), z.unknown()),
	app: z.object({
		openAtLogin: z.boolean(),
		hasSeenIntro: z.boolean(),
		hasCompletedSetup: z.boolean(),
		hasDismissedSetupWizard: z.boolean(),
		personaId: z.string().min(1),
		personas: z.object({
			custom: z.array(
				z.object({
					id: z.string().min(1),
					name: z.string().min(1),
					avatarPath: z.string(),
					soulPath: z.string().min(1),
					createdAt: z.number()
				})
			)
		}),
		miniWindowSessionId: z.string(),
		miniWindowLastActiveAt: z.number().int().min(0),
		miniWindowInactivityTimeoutMinutes: z
			.number()
			.int()
			.min(1)
			.max(24 * 60),
		webPanelWidth: z.number().int().min(0)
	}),
	cometmind: z.object({
		systemPromptPath: z.string(),
		maxTokens: z.number().int().positive(),
		logLevel: z.enum(['debug', 'info', 'warn', 'error']),
		contextWindowLimit: z.union([z.literal(128_000), z.literal(256_000)]),
		titleProviderId: z.string(),
		titleModelId: z.string(),
		acp: z.object({
			command: z.string().min(1),
			args: z.array(z.string()),
			timeout: z.string().min(1)
		}),
		skills: z.object({
			enabled: z.boolean(),
			roots: z.array(z.string()),
			includeOpenCode: z.boolean(),
			includeClaude: z.boolean(),
			mirrorToCometMind: z.boolean()
		}),
		memory: z.object({
			extractionProviderId: z.string(),
			extractionModel: z.string(),
			embedding: z.object({
				providerId: z.string(),
				provider: z.string(),
				model: z.string(),
				baseURL: z.string(),
				apiKey: z.string()
			})
		}),
		storage: z.object({
			cleanupIntervalMinutes: z.number().int().min(0),
			retentionDays: z.number().int().min(0),
			maxSessionsPerWorkspace: z.number().int().min(0),
			archivedMemoryPurgeDays: z.number().int().min(0),
			deletedJobPurgeDays: z.number().int().min(0),
			vacuumAfterPurge: z.boolean()
		}),
		gateway: z.object({
			discord: z.object({
				enabled: z.boolean(),
				botToken: z.string(),
				botTokenEnv: z.string(),
				providerId: z.string(),
				modelId: z.string(),
				allowedUsers: z.array(z.string()),
				allowedChannels: z.array(z.string()),
				requireMention: z.boolean(),
				workspacePath: z.string()
			})
		}),
		mcp: z.object({
			enabled: z.boolean(),
			servers: z.array(
				z.object({
					id: z.string().min(1),
					name: z.string(),
					enabled: z.boolean(),
					transport: z.enum(['stdio', 'http', 'sse']),
					command: z.string().optional(),
					args: z.array(z.string()).optional(),
					env: z.record(z.string(), z.string()).optional(),
					url: z.string().optional(),
					headers: z.record(z.string(), z.string()).optional(),
					oauth: z
						.object({
							clientId: z.string().optional(),
							scopes: z.array(z.string()).optional(),
							authorizationUrl: z.string().optional(),
							tokenUrl: z.string().optional()
						})
						.optional(),
					allowedTools: z.array(z.string()).optional()
				})
			)
		}),
		jobs: z.object({
			notifications: z.object({
				enabled: z.boolean(),
				onClaimed: z.boolean(),
				onCompleted: z.boolean(),
				onReleased: z.boolean()
			}),
			leaseMinutes: z.number().int().positive(),
			deletedPurgeDays: z.number().int().min(0),
			reconcileIntervalSeconds: z.number().int().positive()
		})
	})
});

export class SettingsValidationError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'SettingsValidationError';
	}
}

export function validateSettings(settings: ProviderSettings): ProviderSettings {
	const result = providerSettingsSchema.safeParse(settings);
	if (!result.success) {
		const detail = result.error.issues.map((i) => i.path.join('.')).join(', ');
		throw new SettingsValidationError(`Invalid settings: ${detail}`);
	}
	return settings;
}

export function parseAndNormalizeSettings(
	raw: unknown,
	options: NormalizeSettingsOptions = {}
): ProviderSettings {
	if (!raw || typeof raw !== 'object') {
		return validateSettings(normalizeSettings(defaultSettings(), options));
	}
	const record = raw as Record<string, unknown>;
	const migrated = migrateSingleProvider(record);
	const partial = raw as Partial<ProviderSettings>;
	const base = migrated
		? { ...defaultSettings(), ...partial, ...migrated }
		: { ...defaultSettings(), ...partial };
	return validateSettings(normalizeSettings(base, options));
}
