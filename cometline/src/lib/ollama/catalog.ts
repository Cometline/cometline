/** Static, versioned Ollama model catalog shipped with the app. */

export type OllamaCatalogRole = 'chat' | 'title' | 'embedding';

export interface OllamaCatalogCapabilities {
	chat: boolean;
	title: boolean;
	embedding: boolean;
	/** Not advertised until Phase 0 agent validation passes. */
	agent: boolean;
	/** Not advertised until JSON-only extraction validation passes. */
	extraction: boolean;
}

export interface OllamaCatalogEntry {
	id: string;
	displayName: string;
	pullName: string;
	sizeEstimateBytes: number;
	sizeLabel: string;
	description: string;
	roles: OllamaCatalogRole[];
	capabilities: OllamaCatalogCapabilities;
	featured: boolean;
	licenseURL?: string;
	architectureNote?: string;
}

export const OLLAMA_DEFAULT_NATIVE_BASE = 'http://127.0.0.1:11434';
export const OLLAMA_DOWNLOAD_URL = 'https://ollama.com/download';
export const OLLAMA_CATALOG_VERSION = 1;

export const OLLAMA_MODEL_NAME_RE = /^[a-zA-Z0-9]([a-zA-Z0-9._:-]{0,198}[a-zA-Z0-9])?$/;

export const OLLAMA_CATALOG: OllamaCatalogEntry[] = [
	{
		id: 'private-memory',
		displayName: 'Private Memory',
		pullName: 'qwen3-embedding:0.6b',
		sizeEstimateBytes: 639 * 1024 * 1024,
		sizeLabel: '~639 MB',
		description: 'qwen3-embedding:0.6b',
		roles: ['embedding'],
		capabilities: {
			chat: false,
			title: false,
			embedding: true,
			agent: false,
			extraction: false
		},
		featured: true,
		licenseURL: 'https://ollama.com/library/qwen3-embedding'
	},
	{
		id: 'local-companion-efficient',
		displayName: 'Local Companion — Efficient',
		pullName: 'gemma4:e2b-mlx',
		sizeEstimateBytes: Math.round(6.5 * 1024 * 1024 * 1024),
		sizeLabel: '~6.5 GB',
		description: 'gemma4:e2b-mlx',
		roles: ['chat', 'title'],
		capabilities: {
			chat: true,
			title: true,
			embedding: false,
			agent: false,
			extraction: false
		},
		featured: true,
		architectureNote: 'Apple Silicon (MLX)',
		licenseURL: 'https://ollama.com/library/gemma4'
	},
	{
		id: 'local-companion-better',
		displayName: 'Local Companion — Better quality',
		pullName: 'gemma4:e4b-mlx',
		sizeEstimateBytes: Math.round(8.8 * 1024 * 1024 * 1024),
		sizeLabel: '~8.8 GB',
		description: 'gemma4:e4b-mlx',
		roles: ['chat', 'title'],
		capabilities: {
			chat: true,
			title: true,
			embedding: false,
			agent: false,
			extraction: false
		},
		featured: true,
		architectureNote: 'Apple Silicon (MLX)',
		licenseURL: 'https://ollama.com/library/gemma4'
	},
	{
		id: 'private-memory-lightweight',
		displayName: 'Private Memory — Lightweight',
		pullName: 'embeddinggemma',
		sizeEstimateBytes: 622 * 1024 * 1024,
		sizeLabel: '~622 MB',
		description: 'embeddinggemma',
		roles: ['embedding'],
		capabilities: {
			chat: false,
			title: false,
			embedding: true,
			agent: false,
			extraction: false
		},
		featured: false,
		licenseURL: 'https://ollama.com/library/embeddinggemma'
	}
];

export function featuredOllamaCatalog(): OllamaCatalogEntry[] {
	return OLLAMA_CATALOG.filter((entry) => entry.featured);
}

export function getOllamaCatalogEntry(id: string): OllamaCatalogEntry | undefined {
	return OLLAMA_CATALOG.find((entry) => entry.id === id);
}

export function getOllamaCatalogByPullName(pullName: string): OllamaCatalogEntry | undefined {
	const trimmed = pullName.trim();
	return OLLAMA_CATALOG.find((entry) => entry.pullName === trimmed);
}

export function isValidOllamaModelName(name: string): boolean {
	const trimmed = name.trim();
	if (!trimmed || trimmed.length > 200) return false;
	if (trimmed.includes('://') || trimmed.includes('/') || trimmed.includes('\\')) return false;
	return OLLAMA_MODEL_NAME_RE.test(trimmed);
}
