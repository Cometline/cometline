import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	apiErrorMessage,
	streamMessage,
	streamSessionEvents,
	UnexpectedStreamEndError
} from './cometmind';

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

describe('apiErrorMessage', () => {
	it('extracts a simple backend error response', () => {
		expect(apiErrorMessage({ error: 'backup destination is unavailable' }, 'Backup failed')).toBe(
			'backup destination is unavailable'
		);
	});

	it('extracts a structured backend error response', () => {
		expect(
			apiErrorMessage(
				{ error: { code: 'backup_failed', message: 'database snapshot failed' } },
				'Backup failed'
			)
		).toBe('database snapshot failed');
	});

	it('prefers an actionable lifecycle hint over the raw error', () => {
		expect(
			apiErrorMessage(
				{ error: 'connect: context deadline exceeded', error_hint: 'Click Reconnect.' },
				'Reconnect failed'
			)
		).toBe('Click Reconnect.');
	});

	it('uses the fallback for unknown values', () => {
		expect(apiErrorMessage({ nope: true }, 'Backup failed')).toBe('Backup failed');
	});
});

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

describe('streamSessionEvents', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('uses GET and yields replay through the terminal event', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			sseResponse(
				'data: {"type":"text_delta","delta":"replayed"}\n\n' +
					'data: {"type":"done"}\n\n'
			)
		);
		vi.stubGlobal('fetch', fetchMock);

		const events = [];
		for await (const event of streamSessionEvents('session/1')) events.push(event);

		expect(fetchMock).toHaveBeenCalledWith(
			'http://127.0.0.1:7700/api/v1/sessions/session%2F1/events',
			expect.objectContaining({ method: 'GET', cache: 'no-store' })
		);
		expect(events).toEqual([
			{ type: 'text_delta', delta: 'replayed' },
			{ type: 'done' }
		]);
	});
});
