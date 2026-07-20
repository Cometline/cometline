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
import { normalizeOllamaNativeBase } from '../ollama/url';

export const VALID_PROVIDER_METHODS: ProviderMethod[] = [
	'openai-compatible',
	'openai',
	'anthropic',
	'opencode-go',
	'codex',
	'xai',
	'ollama'
];

const BUILTIN_PROVIDER_NAMES: Record<string, string> = {
	'openai-compatible': 'Advanced / Custom endpoint',
	anthropic: 'Anthropic',
	openai: 'OpenAI',
	'opencode-go': 'OpenCode Go',
	codex: 'ChatGPT Codex',
	xai: 'xAI Grok Subscription',
	ollama: 'Ollama Local'
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

export type CodingHarness = 'opencode' | 'claude' | 'codex';

export interface CometMindACPSettings {
	/** When false, delegate_coding_task is not registered (native tools only). */
	enabled: boolean;
	defaultHarness: CodingHarness;
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
	synthesisEnabled: boolean;
	synthesisProviderId: string;
	synthesisModel: string;
}

export interface CometMindMemorySettings {
	enabled: boolean;
	autoExtract: boolean;
	autoRetrieve: boolean;
	maxRetrieved: number;
	taskOutcomeLimit: number;
	similarityThreshold: number;
	extractionProviderId: string;
	extractionModel: string;
	lifecycle: {
		decayHalfLifeDays: number;
		forgetThreshold: number;
		usageBoostFactor: number;
		maxUsageBoost: number;
		maxMemories: number;
		compactionTargetRatio: number;
		compactionOnExtract: boolean;
	};
	embedding: {
		providerId: string;
		provider: string;
		model: string;
		baseURL: string;
		apiKey: string;
	};
}

export interface CometMindStorageBackupSettings {
	enabled: boolean;
	destinationDir: string;
	intervalHours: number;
	maxBackups: number;
}

export interface CometMindStorageSettings {
	cleanupIntervalMinutes: number;
	retentionDays: number;
	maxSessionsPerWorkspace: number;
	archivedMemoryPurgeDays: number;
	deletedJobPurgeDays: number;
	vacuumAfterPurge: boolean;
	/** Delete ~/.cometmind/tool-output files older than N days. 0 disables. */
	toolOutputRetentionDays: number;
	/** Delete ~/.cometmind/agent-tmp files older than N days. 0 disables. */
	agentTmpRetentionDays: number;
	backup: CometMindStorageBackupSettings;
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
	onBlocked: boolean;
}

export interface CometMindJobsSettings {
	notifications: CometMindJobsNotificationSettings;
	leaseMinutes: number;
	deletedPurgeDays: number;
	doneArchiveDays: number;
	archivedPurgeDays: number;
	staleReviewMinutes: number;
	maxConsecutiveFailures: number;
	retryCooldownMinutes: number;
	maxRetryCooldownMinutes: number;
	reconcileIntervalSeconds: number;
}

export interface CometMindAutonomousJobsSettings {
	enabled: boolean;
	maxConcurrent: number;
	pollIntervalSeconds: number;
	maxStepsPerRun: number;
	providerId: string;
	modelId: string;
}

export interface CometMindSchedulerSettings {
	enabled: boolean;
	pollIntervalSeconds: number;
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
	autonomy: CometMindAutonomousJobsSettings;
	scheduler: CometMindSchedulerSettings;
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
		id: 'xai',
		name: 'xAI Grok Subscription',
		method: 'xai',
		enabled: false,
		baseURL: 'https://api.x.ai/v1',
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
		id: 'ollama',
		name: 'Ollama Local',
		method: 'ollama',
		enabled: false,
		baseURL: 'http://127.0.0.1:11434',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'openai-compatible',
		name: 'Advanced / Custom endpoint',
		method: 'openai-compatible',
		enabled: false,
		baseURL: '',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	}
];

const FIXED_BUILTIN_PROVIDER_IDS = new Set([
	'codex',
	'xai',
	'openai',
	'anthropic',
	'opencode-go',
	'ollama'
]);

/** Built-in integrations have a stable identity; custom endpoints remain configurable. */
export function isFixedBuiltinProvider(id: string): boolean {
	return FIXED_BUILTIN_PROVIDER_IDS.has(id);
}

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

function normalizePositiveNumber(value: unknown, fallback: number): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) return fallback;
	return Math.max(Number.EPSILON, value);
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
			onReleased: false,
			onBlocked: true
		},
		leaseMinutes: 30,
		deletedPurgeDays: 30,
		doneArchiveDays: 3,
		archivedPurgeDays: 30,
		staleReviewMinutes: 30,
		maxConsecutiveFailures: 3,
		retryCooldownMinutes: 5,
		maxRetryCooldownMinutes: 60,
		reconcileIntervalSeconds: 120
	};
}

export function defaultCometMindAutonomousJobsSettings(): CometMindAutonomousJobsSettings {
	return {
		enabled: false,
		maxConcurrent: 1,
		pollIntervalSeconds: 30,
		maxStepsPerRun: 0,
		providerId: '',
		modelId: ''
	};
}

export function defaultCometMindSchedulerSettings(): CometMindSchedulerSettings {
	return { enabled: false, pollIntervalSeconds: 60 };
}

export function defaultCometMindStorageBackupSettings(): CometMindStorageBackupSettings {
	return {
		enabled: false,
		destinationDir: '',
		intervalHours: 24,
		maxBackups: 7
	};
}

export function defaultCometMindStorageSettings(): CometMindStorageSettings {
	return {
		cleanupIntervalMinutes: 60,
		retentionDays: 90,
		maxSessionsPerWorkspace: 0,
		archivedMemoryPurgeDays: 90,
		deletedJobPurgeDays: 30,
		vacuumAfterPurge: true,
		toolOutputRetentionDays: 7,
		agentTmpRetentionDays: 3,
		backup: defaultCometMindStorageBackupSettings()
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
			enabled: false,
			defaultHarness: 'opencode'
		},
		skills: {
			enabled: true,
			roots: [],
			includeOpenCode: true,
			includeClaude: true,
			mirrorToCometMind: true,
			synthesisEnabled: false,
			synthesisProviderId: '',
			synthesisModel: ''
		},
		memory: {
			enabled: true,
			autoExtract: true,
			autoRetrieve: true,
			maxRetrieved: 5,
			taskOutcomeLimit: 3,
			similarityThreshold: 0.5,
			extractionProviderId: '',
			extractionModel: '',
			lifecycle: {
				decayHalfLifeDays: 30,
				forgetThreshold: 0.1,
				usageBoostFactor: 0.15,
				maxUsageBoost: 2,
				maxMemories: 500,
				compactionTargetRatio: 0.8,
				compactionOnExtract: true
			},
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
		jobs: defaultCometMindJobsSettings(),
		autonomy: defaultCometMindAutonomousJobsSettings(),
		scheduler: defaultCometMindSchedulerSettings()
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
	const memoryLifecycle: Partial<CometMindMemorySettings['lifecycle']> = memory.lifecycle ?? {};
	const embedding: Partial<CometMindMemorySettings['embedding']> = memory.embedding ?? {};
	const storage: Partial<CometMindStorageSettings> = input?.storage ?? {};
	const discord: Partial<CometMindDiscordGatewaySettings> = input?.gateway?.discord ?? {};
	const mcp = normalizeCometMindMCPSettings(input?.mcp);
	const jobsInput: Partial<CometMindJobsSettings> = input?.jobs ?? {};
	const jobsDefaults = defaults.jobs;
	const jobsNotifications: Partial<CometMindJobsNotificationSettings> =
		jobsInput.notifications ?? {};
	const autonomyInput: Partial<CometMindAutonomousJobsSettings> = input?.autonomy ?? {};
	const autonomyDefaults = defaults.autonomy;
	const schedulerInput: Partial<CometMindSchedulerSettings> = input?.scheduler ?? {};
	const normalizeHarness = (value: unknown): CodingHarness => {
		const raw = String(value ?? '').trim();
		return raw === 'claude' || raw === 'codex' || raw === 'opencode'
			? raw
			: defaults.acp.defaultHarness;
	};
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
			enabled: typeof acp.enabled === 'boolean' ? acp.enabled : defaults.acp.enabled,
			defaultHarness: normalizeHarness(acp.defaultHarness)
		},
		skills: {
			enabled: typeof skills.enabled === 'boolean' ? skills.enabled : defaults.skills.enabled,
			roots: [],
			includeOpenCode: true,
			includeClaude: true,
			mirrorToCometMind: true,
			synthesisEnabled:
				typeof skills.synthesisEnabled === 'boolean'
					? skills.synthesisEnabled
					: defaults.skills.synthesisEnabled,
			synthesisProviderId: String(
				skills.synthesisProviderId ?? defaults.skills.synthesisProviderId
			).trim(),
			synthesisModel: String(skills.synthesisModel ?? defaults.skills.synthesisModel).trim()
		},
		memory: {
			enabled: typeof memory.enabled === 'boolean' ? memory.enabled : defaults.memory.enabled,
			autoExtract:
				typeof memory.autoExtract === 'boolean'
					? memory.autoExtract
					: defaults.memory.autoExtract,
			autoRetrieve:
				typeof memory.autoRetrieve === 'boolean'
					? memory.autoRetrieve
					: defaults.memory.autoRetrieve,
			maxRetrieved: normalizePositiveInt(memory.maxRetrieved, defaults.memory.maxRetrieved),
			taskOutcomeLimit: normalizePositiveInt(
				memory.taskOutcomeLimit,
				defaults.memory.taskOutcomeLimit
			),
			similarityThreshold: normalizeUnit(
				memory.similarityThreshold,
				defaults.memory.similarityThreshold
			),
			extractionProviderId: String(
				memory.extractionProviderId ?? defaults.memory.extractionProviderId
			).trim(),
			extractionModel: String(
				memory.extractionModel ?? defaults.memory.extractionModel
			).trim(),
			lifecycle: {
				decayHalfLifeDays: normalizePositiveInt(
					memoryLifecycle.decayHalfLifeDays,
					defaults.memory.lifecycle.decayHalfLifeDays
				),
				forgetThreshold: normalizeUnit(
					memoryLifecycle.forgetThreshold,
					defaults.memory.lifecycle.forgetThreshold
				),
				usageBoostFactor: normalizeUnit(
					memoryLifecycle.usageBoostFactor,
					defaults.memory.lifecycle.usageBoostFactor
				),
				maxUsageBoost: normalizePositiveNumber(
					memoryLifecycle.maxUsageBoost,
					defaults.memory.lifecycle.maxUsageBoost
				),
				maxMemories: normalizePositiveInt(
					memoryLifecycle.maxMemories,
					defaults.memory.lifecycle.maxMemories
				),
				compactionTargetRatio: normalizeUnit(
					memoryLifecycle.compactionTargetRatio,
					defaults.memory.lifecycle.compactionTargetRatio
				),
				compactionOnExtract:
					typeof memoryLifecycle.compactionOnExtract === 'boolean'
						? memoryLifecycle.compactionOnExtract
						: defaults.memory.lifecycle.compactionOnExtract
			},
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
					: defaults.storage.vacuumAfterPurge,
			toolOutputRetentionDays: normalizeNonNegativeInt(
				storage.toolOutputRetentionDays,
				defaults.storage.toolOutputRetentionDays
			),
			agentTmpRetentionDays: normalizeNonNegativeInt(
				storage.agentTmpRetentionDays,
				defaults.storage.agentTmpRetentionDays
			),
			backup: {
				enabled:
					typeof storage.backup?.enabled === 'boolean'
						? storage.backup.enabled
						: defaults.storage.backup.enabled,
				destinationDir: String(
					storage.backup?.destinationDir ?? defaults.storage.backup.destinationDir
				).trim(),
				intervalHours: normalizePositiveInt(
					storage.backup?.intervalHours,
					defaults.storage.backup.intervalHours
				),
				maxBackups: normalizeNonNegativeInt(
					storage.backup?.maxBackups,
					defaults.storage.backup.maxBackups
				)
			}
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
						: jobsDefaults.notifications.onReleased,
				onBlocked:
					typeof jobsNotifications.onBlocked === 'boolean'
						? jobsNotifications.onBlocked
						: jobsDefaults.notifications.onBlocked
			},
			leaseMinutes: normalizePositiveInt(jobsInput.leaseMinutes, jobsDefaults.leaseMinutes),
			deletedPurgeDays: normalizeNonNegativeInt(
				jobsInput.deletedPurgeDays,
				jobsDefaults.deletedPurgeDays
			),
			doneArchiveDays: normalizeNonNegativeInt(
				jobsInput.doneArchiveDays,
				jobsDefaults.doneArchiveDays
			),
			archivedPurgeDays: normalizeNonNegativeInt(
				jobsInput.archivedPurgeDays,
				jobsDefaults.archivedPurgeDays
			),
			staleReviewMinutes: normalizePositiveInt(
				jobsInput.staleReviewMinutes,
				jobsDefaults.staleReviewMinutes
			),
			maxConsecutiveFailures: normalizePositiveInt(
				jobsInput.maxConsecutiveFailures,
				jobsDefaults.maxConsecutiveFailures
			),
			retryCooldownMinutes: normalizePositiveInt(
				jobsInput.retryCooldownMinutes,
				jobsDefaults.retryCooldownMinutes
			),
			maxRetryCooldownMinutes: normalizePositiveInt(
				jobsInput.maxRetryCooldownMinutes,
				jobsDefaults.maxRetryCooldownMinutes
			),
			reconcileIntervalSeconds: normalizePositiveInt(
				jobsInput.reconcileIntervalSeconds,
				jobsDefaults.reconcileIntervalSeconds
			)
		},
		autonomy: {
			enabled:
				typeof autonomyInput.enabled === 'boolean'
					? autonomyInput.enabled
					: autonomyDefaults.enabled,
			maxConcurrent: normalizePositiveInt(
				autonomyInput.maxConcurrent,
				autonomyDefaults.maxConcurrent
			),
			pollIntervalSeconds: normalizePositiveInt(
				autonomyInput.pollIntervalSeconds,
				autonomyDefaults.pollIntervalSeconds
			),
			maxStepsPerRun: normalizeNonNegativeInt(
				autonomyInput.maxStepsPerRun,
				autonomyDefaults.maxStepsPerRun
			),
			providerId: String(autonomyInput.providerId ?? autonomyDefaults.providerId).trim(),
			modelId: String(autonomyInput.modelId ?? autonomyDefaults.modelId).trim()
		},
		scheduler: {
			enabled:
				typeof schedulerInput.enabled === 'boolean'
					? schedulerInput.enabled
					: defaults.scheduler.enabled,
			pollIntervalSeconds: normalizePositiveInt(
				schedulerInput.pollIntervalSeconds,
				defaults.scheduler.pollIntervalSeconds
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
			enabled: settings.acp.enabled,
			defaultHarness: settings.acp.defaultHarness
		},
		skills: {
			...settings.skills,
			roots: [...settings.skills.roots]
		},
		memory: {
			enabled: settings.memory.enabled,
			autoExtract: settings.memory.autoExtract,
			autoRetrieve: settings.memory.autoRetrieve,
			maxRetrieved: settings.memory.maxRetrieved,
			taskOutcomeLimit: settings.memory.taskOutcomeLimit,
			similarityThreshold: settings.memory.similarityThreshold,
			extractionProviderId: settings.memory.extractionProviderId,
			extractionModel: settings.memory.extractionModel,
			lifecycle: { ...settings.memory.lifecycle },
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
		},
		autonomy: { ...settings.autonomy },
		scheduler: { ...settings.scheduler }
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
		webPanelWidth: 0,
		confirmCloseOnCmdW: true,
		confirmBeforeDeletingChats: true
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
	const id = String(
		provider.id ||
			fallback?.id ||
			`provider-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
	).trim();
	const fixedBuiltin = isFixedBuiltinProvider(id)
		? DEFAULT_PROVIDERS.find((candidate) => candidate.id === id)
		: undefined;
	const method = fixedBuiltin
		? fixedBuiltin.method
		: VALID_PROVIDER_METHODS.includes(provider.method as ProviderMethod)
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
	let baseURL = String(provider.baseURL ?? fallback?.baseURL ?? '').trim();
	if (method === 'ollama') {
		baseURL = normalizeOllamaNativeBase(baseURL || fixedBuiltin?.baseURL);
	}
	return {
		id,
		name: fixedBuiltin?.name ?? providerNameOrDefault(provider, fallback, id),
		method,
		enabled:
			typeof provider.enabled === 'boolean' ? provider.enabled : Boolean(fallback?.enabled),
		baseURL,
		apiKey:
			method === 'codex' || method === 'xai' || method === 'ollama'
				? ''
				: String(provider.apiKey ?? fallback?.apiKey ?? '').trim(),
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
	const { defaultProviderId, defaultModelId, activeProviderId } = resolveDefaultModelPair(
		providers,
		next.defaultProviderId,
		next.defaultModelId,
		next.activeProviderId
	);
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
		// Mirror Default into activeProviderId for legacy Electron/env readers.
		activeProviderId,
		defaultModelId,
		defaultProviderId,
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
			webPanelWidth: normalizeWebPanelWidth(next.app?.webPanelWidth),
			confirmCloseOnCmdW:
				typeof next.app?.confirmCloseOnCmdW === 'boolean'
					? next.app.confirmCloseOnCmdW
					: defaultAppSettings().confirmCloseOnCmdW,
			confirmBeforeDeletingChats:
				typeof next.app?.confirmBeforeDeletingChats === 'boolean'
					? next.app.confirmBeforeDeletingChats
					: defaultAppSettings().confirmBeforeDeletingChats
		},
		cometmind
	};
}

/** Resolve Default model pair; migrate from legacy activeProviderId when Default is empty. */
export function resolveDefaultModelPair(
	providers: ProviderConfig[],
	preferredDefaultProviderId?: string,
	preferredDefaultModelId?: string,
	legacyActiveProviderId?: string
): { defaultProviderId: string; defaultModelId: string; activeProviderId: string } {
	const runtime = providers.filter((p) => p.enabled && p.enabledModels.length > 0);
	const byId = new Map(runtime.map((p) => [p.id, p]));

	let defaultProviderId = String(preferredDefaultProviderId ?? '').trim();
	let defaultModelId = String(preferredDefaultModelId ?? '').trim();

	if (defaultProviderId && byId.has(defaultProviderId)) {
		const provider = byId.get(defaultProviderId)!;
		if (!defaultModelId || !provider.enabledModels.includes(defaultModelId)) {
			defaultModelId = primaryModel(provider);
		}
	} else {
		const legacyActive = String(legacyActiveProviderId ?? '').trim();
		const migrated =
			(legacyActive && byId.get(legacyActive)) || runtime[0] || providers[0] || null;
		defaultProviderId = migrated?.id ?? '';
		defaultModelId = migrated ? primaryModel(migrated) : '';
	}

	return {
		defaultProviderId,
		defaultModelId,
		// Keep active mirrored to Default so legacy env/readers stay consistent.
		activeProviderId: defaultProviderId
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
		providers.find((p) => p.id === settings.defaultProviderId) ??
		providers.find((p) => p.id === settings.activeProviderId) ??
		providers[0] ??
		null;
	if (!active) return null;

	const model =
		(settings.defaultModelId &&
		active.enabledModels.includes(settings.defaultModelId)
			? settings.defaultModelId
			: primaryModel(active));

	return {
		provider: active.id,
		model,
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
		acp: { ...settings.cometmind.acp },
		skills: { ...settings.cometmind.skills, roots: [...settings.cometmind.skills.roots] },
		memory: {
			enabled: settings.cometmind.memory.enabled,
			autoExtract: settings.cometmind.memory.autoExtract,
			autoRetrieve: settings.cometmind.memory.autoRetrieve,
			maxRetrieved: settings.cometmind.memory.maxRetrieved,
			taskOutcomeLimit: settings.cometmind.memory.taskOutcomeLimit,
			similarityThreshold: settings.cometmind.memory.similarityThreshold,
			extractionProviderId: settings.cometmind.memory.extractionProviderId,
			extractionModel: settings.cometmind.memory.extractionModel,
			lifecycle: { ...settings.cometmind.memory.lifecycle },
			embedding: { ...settings.cometmind.memory.embedding }
		},
		gateway: cloneCometMindSettings(settings.cometmind).gateway,
		mcp: cloneCometMindSettings(settings.cometmind).mcp
	};
}

const providerConfigSchema = z.object({
	id: z.string().min(1),
	name: z.string(),
	method: z.enum([
		'openai-compatible',
		'openai',
		'anthropic',
		'opencode-go',
		'codex',
		'xai',
		'ollama'
	]),
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
			presetId: z.enum(['blue', 'rose', 'custom']),
			glowColor: z.string(),
			ringColor: z.string(),
			customPreset: z
				.object({
					glowColor: z.string(),
					ringColor: z.string()
				})
				.optional()
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
		webPanelWidth: z.number().int().min(0),
		confirmCloseOnCmdW: z.boolean(),
		confirmBeforeDeletingChats: z.boolean()
	}),
	cometmind: z.object({
		systemPromptPath: z.string(),
		maxTokens: z.number().int().positive(),
		logLevel: z.enum(['debug', 'info', 'warn', 'error']),
		contextWindowLimit: z.union([z.literal(128_000), z.literal(256_000)]),
		titleProviderId: z.string(),
		titleModelId: z.string(),
		acp: z.object({
			enabled: z.boolean(),
			defaultHarness: z.enum(['opencode', 'claude', 'codex'])
		}),
		skills: z.object({
			enabled: z.boolean(),
			roots: z.array(z.string()),
			includeOpenCode: z.boolean(),
			includeClaude: z.boolean(),
			mirrorToCometMind: z.boolean(),
			synthesisEnabled: z.boolean(),
			synthesisProviderId: z.string(),
			synthesisModel: z.string()
		}),
		memory: z.object({
			enabled: z.boolean(),
			autoExtract: z.boolean(),
			autoRetrieve: z.boolean(),
			maxRetrieved: z.number().int().positive(),
			taskOutcomeLimit: z.number().int().positive(),
			similarityThreshold: z.number().min(0).max(1),
			extractionProviderId: z.string(),
			extractionModel: z.string(),
			lifecycle: z.object({
				decayHalfLifeDays: z.number().positive(),
				forgetThreshold: z.number().min(0).max(1),
				usageBoostFactor: z.number().min(0).max(1),
				maxUsageBoost: z.number().positive(),
				maxMemories: z.number().int().positive(),
				compactionTargetRatio: z.number().min(0).max(1),
				compactionOnExtract: z.boolean()
			}),
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
			vacuumAfterPurge: z.boolean(),
			toolOutputRetentionDays: z.number().int().min(0),
			agentTmpRetentionDays: z.number().int().min(0),
			backup: z.object({
				enabled: z.boolean(),
				destinationDir: z.string(),
				intervalHours: z.number().int().min(1),
				maxBackups: z.number().int().min(0)
			})
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
				onReleased: z.boolean(),
				onBlocked: z.boolean()
			}),
			leaseMinutes: z.number().int().positive(),
			deletedPurgeDays: z.number().int().min(0),
			doneArchiveDays: z.number().int().min(0),
			archivedPurgeDays: z.number().int().min(0),
			staleReviewMinutes: z.number().int().positive(),
			maxConsecutiveFailures: z.number().int().positive(),
			retryCooldownMinutes: z.number().int().positive(),
			maxRetryCooldownMinutes: z.number().int().positive(),
			reconcileIntervalSeconds: z.number().int().positive()
		}),
		autonomy: z.object({
			enabled: z.boolean(),
			maxConcurrent: z.number().int().positive(),
			pollIntervalSeconds: z.number().int().positive(),
			maxStepsPerRun: z.number().int().min(0),
			providerId: z.string(),
			modelId: z.string()
		}),
		scheduler: z.object({
			enabled: z.boolean(),
			pollIntervalSeconds: z.number().int().positive()
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
