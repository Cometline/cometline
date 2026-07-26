export type MemoryBucket = 'preference' | 'task_outcome' | 'semantic';

export const MEMORY_BUCKETS = [
	{ bucket: 'preference', label: 'User preferences' },
	{ bucket: 'task_outcome', label: 'Relevant task outcomes' },
	{ bucket: 'semantic', label: 'Semantic memories' }
] as const satisfies ReadonlyArray<{ bucket: MemoryBucket; label: string }>;

export function inferMemoryBucket(kind: string): MemoryBucket {
	if (kind === 'preference') return 'preference';
	if (kind === 'task_outcome' || kind === 'task_summary') return 'task_outcome';
	return 'semantic';
}

export function resolveMemoryBucket(memory: { kind: string; bucket?: MemoryBucket }): MemoryBucket {
	return memory.bucket ?? inferMemoryBucket(memory.kind);
}

export function bucketMemories<T extends { kind: string; bucket?: MemoryBucket }>(
	memories: readonly T[]
) {
	return MEMORY_BUCKETS.map(({ bucket, label }) => ({
		bucket,
		label,
		memories: memories.filter((memory) => resolveMemoryBucket(memory) === bucket)
	})).filter((section) => section.memories.length > 0);
}

export function memoryKindLabel(kind: string): string {
	return kind.replaceAll('_', ' ');
}
