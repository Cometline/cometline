export interface JobProposal {
	description: string;
	definitionOfDone: string;
	defaultWorkspace: string;
}

function readString(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

function fromRecord(record: Record<string, unknown>): JobProposal | null {
	const description = readString(record.description);
	if (!description) return null;
	return {
		description,
		definitionOfDone: readString(record.definition_of_done),
		defaultWorkspace: readString(record.default_workspace)
	};
}

function parseJsonObject(raw: string): Record<string, unknown> | null {
	try {
		const parsed: unknown = JSON.parse(raw);
		if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
			return parsed as Record<string, unknown>;
		}
	} catch {
		return null;
	}
	return null;
}

/** Parse a propose_job tool input or output into a job proposal. */
export function parseJobProposal(input: unknown, output?: string): JobProposal | null {
	if (input && typeof input === 'object' && !Array.isArray(input)) {
		const fromInput = fromRecord(input as Record<string, unknown>);
		if (fromInput) {
			if (output?.trim()) {
				const fromOutput = parseJsonObject(output.trim());
				if (fromOutput) {
					return {
						...fromInput,
						defaultWorkspace:
							readString(fromOutput.default_workspace) || fromInput.defaultWorkspace
					};
				}
			}
			return fromInput;
		}
	}
	if (typeof input === 'string' && input.trim()) {
		const fromInput = parseJsonObject(input.trim());
		if (fromInput) {
			const proposal = fromRecord(fromInput);
			if (proposal) return proposal;
		}
	}
	if (output?.trim()) {
		const fromOutput = parseJsonObject(output.trim());
		if (fromOutput) return fromRecord(fromOutput);
	}
	return null;
}
