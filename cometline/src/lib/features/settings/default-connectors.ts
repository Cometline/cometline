import type { MCPServerConfig } from '#lib/cometmind-settings.js';

const CATALOG_URL =
	'https://raw.githubusercontent.com/Cometline/cometline-connector/main/connectors/catalog.json';
const CACHE_KEY = 'cometline.connector-catalog';

export interface DefaultConnectorMode {
	id: 'remote' | 'local';
	transport: 'http' | 'stdio';
	url?: string;
	command?: string;
	args?: string[];
	auth: 'none' | 'token' | 'oauth-or-token';
	requirements?: string[];
	source: string;
}

export interface DefaultConnector {
	id: string;
	name: string;
	description: string;
	iconSvg: string;
	verifiedAt: string;
	available?: boolean;
	unavailableReason?: string;
	modes: DefaultConnectorMode[];
}

interface CatalogDocument {
	version: string;
	connectors: DefaultConnector[];
}

export function defaultConnectorServerId(connectorId: string): string {
	return `connector-${connectorId}`;
}

export function connectorServerConfig(connector: DefaultConnector): MCPServerConfig {
	const mode = connector.modes[0];
	if (!mode) {
		throw new Error(`Connector ${connector.id} cannot be connected yet`);
	}
	return {
		id: defaultConnectorServerId(connector.id),
		name: connector.name,
		enabled: true,
		transport: mode.transport,
		command: mode.command ?? '',
		args: mode.args ?? [],
		env: {},
		url: mode.url ?? '',
		headers: {}
	};
}

export async function loadConnectorCatalog(): Promise<DefaultConnector[]> {
	try {
		const response = await fetch(CATALOG_URL);
		if (!response.ok) {
			throw new Error(`Catalog request failed: ${response.status}`);
		}
		const connectors = parseCatalog(await response.json());
		localStorage.setItem(CACHE_KEY, JSON.stringify(connectors));
		return connectors;
	} catch (error) {
		const cached = readCachedCatalog();
		if (cached.length > 0) return cached;
		throw error;
	}
}

function parseCatalog(value: unknown): DefaultConnector[] {
	if (!isRecord(value) || !Array.isArray(value.connectors)) {
		throw new Error('Connector catalog has no connectors array');
	}
	return value.connectors.flatMap((item) => {
		const connector = parseConnector(item);
		return connector ? [connector] : [];
	});
}

function parseConnector(value: unknown): DefaultConnector | null {
	if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string') {
		return null;
	}
	const modes = Array.isArray(value.modes)
		? value.modes.flatMap((item) => {
				const mode = parseMode(item);
				return mode ? [mode] : [];
			})
		: [];
	return {
		id: value.id,
		name: value.name,
		description: typeof value.description === 'string' ? value.description : '',
		iconSvg: typeof value.iconSvg === 'string' ? value.iconSvg : '',
		verifiedAt: typeof value.verifiedAt === 'string' ? value.verifiedAt : '',
		available: value.available === false ? false : undefined,
		unavailableReason:
			typeof value.unavailableReason === 'string' ? value.unavailableReason : undefined,
		modes
	};
}

function parseMode(value: unknown): DefaultConnectorMode | null {
	if (!isRecord(value) || (value.transport !== 'http' && value.transport !== 'stdio')) {
		return null;
	}
	return {
		id: value.id === 'local' ? 'local' : 'remote',
		transport: value.transport,
		url: typeof value.url === 'string' ? value.url : undefined,
		command: typeof value.command === 'string' ? value.command : undefined,
		args: Array.isArray(value.args)
			? value.args.filter((item) => typeof item === 'string')
			: undefined,
		auth:
			value.auth === 'token' || value.auth === 'oauth-or-token' || value.auth === 'none'
				? value.auth
				: 'none',
		requirements: Array.isArray(value.requirements)
			? value.requirements.filter((item) => typeof item === 'string')
			: undefined,
		source: typeof value.source === 'string' ? value.source : ''
	};
}

function readCachedCatalog(): DefaultConnector[] {
	try {
		const raw = localStorage.getItem(CACHE_KEY);
		return raw ? parseCatalog(JSON.parse(raw)) : [];
	} catch {
		return [];
	}
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

export type { CatalogDocument };
