import { afterEach, describe, expect, it, vi } from 'vitest';
import { streamMessage, UnexpectedStreamEndError } from './cometmind';

function sseResponse(body: string): Response {
	const encoder = new TextEncoder();
	const stream = new ReadableStream<Uint8Array>({
		start(controller) {
			controller.enqueue(encoder.encode(body));
			controller.close();
		}
	});
	return new Response(stream, {
		status: 200,
		headers: { 'Content-Type': 'text/event-stream' }
	});
}

describe('streamMessage', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('rejects when the SSE connection ends without a done event', async () => {
		vi.stubGlobal(
			'fetch',
			vi
				.fn()
				.mockResolvedValue(sseResponse('data: {"type":"text_delta","delta":"partial"}\n\n'))
		);

		const events: string[] = [];
		const consume = async () => {
			for await (const event of streamMessage('session-1', { text: 'hello' })) {
				events.push(event.type);
			}
		};

		await expect(consume()).rejects.toBeInstanceOf(UnexpectedStreamEndError);
		expect(events).toEqual(['text_delta']);
	});

	it('accepts an explicit done event as a successful terminal frame', async () => {
		vi.stubGlobal(
			'fetch',
			vi
				.fn()
				.mockResolvedValue(
					sseResponse(
						'data: {"type":"text_delta","delta":"complete"}\n\n' +
							'data: {"type":"done"}\n\n'
					)
				)
		);

		const events: string[] = [];
		for await (const event of streamMessage('session-1', { text: 'hello' })) {
			events.push(event.type);
		}

		expect(events).toEqual(['text_delta', 'done']);
	});
});
