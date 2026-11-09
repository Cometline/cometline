import { clamp01, easeOut, seg } from './intro-timeline';
import { INTRO_V2_BEATS as T } from './intro-timeline-v2';

export type IntroV2Motion = {
	windowScale: number;
	windowRadius: number;
	backdropAlpha: number;
	openerOpacity: number;
	wordmarkOpacity: number;
	wordmarkBlur: number;
	iconOpacity: number;
	skyBlur: number;
	skyWash: number;
	skySharpOpacity: number;
	welcomeOpacity: number;
	welcomeScale: number;
	shadowStrength: number;
};

function lerp(a: number, b: number, t: number) {
	return a + (b - a) * t;
}

/** Maps elapsed ms to GPU-friendly motion values for intro v2. */
export function computeIntroV2Motion(elapsed: number, reducedMotion: boolean): IntroV2Motion {
	if (reducedMotion) {
		const t = seg(elapsed, 0, T.reducedTotal);
		return {
			windowScale: 1,
			windowRadius: 16,
			backdropAlpha: 0.2 * t,
			openerOpacity: 1 - t,
			wordmarkOpacity: Math.min(1, t * 1.6) * (1 - t * 0.7),
			wordmarkBlur: 0,
			iconOpacity: t > 0.35 ? 1 : 0,
			skyBlur: lerp(12, 0, easeOut(t)),
			skyWash: 1,
			skySharpOpacity: easeOut(t),
			welcomeOpacity: 0,
			welcomeScale: 1,
			shadowStrength: 0.35
		};
	}

	const windowP = easeOut(seg(elapsed, 0, T.windowSettle));
	const openerFade =
		elapsed < T.openerOut ? 1 - easeOut(seg(elapsed, T.openerHold, T.openerOut)) : 0;
	const wordP = easeOut(seg(elapsed, T.wordmarkIn - 500, T.wordmarkIn + 700));
	const wordHold = 1 - easeOut(seg(elapsed, T.wordmarkHold, T.brandOut));
	const skyP = easeOut(seg(elapsed, T.brandOut, T.skyReveal));
	const welcomeIn = easeOut(seg(elapsed, T.welcomeIn, T.welcomeIn + 900));
	const welcomeOut = easeOut(seg(elapsed, T.welcomeOut, T.welcomeOut + 600));
	const welcomeP = Math.max(0, welcomeIn * (1 - welcomeOut));

	let wordmarkOpacity = clamp01(wordP * wordHold);
	// Icon is the handoff anchor for v1-style flight — visible once the title resolves.
	const iconOpacity = clamp01(wordP * (elapsed >= T.wordmarkIn + 200 ? 1 : 0));
	const openerOpacity = openerFade * Math.max(0, 1 - wordP * 1.5) * Math.max(0, 1 - welcomeP * 2);

	// While the welcome card is up, ease the title back so the panel reads cleanly.
	if (welcomeP > 0.15) {
		wordmarkOpacity *= Math.max(0, 1 - welcomeP * 1.4);
	}

	return {
		windowScale: lerp(1, 0.9, windowP),
		windowRadius: lerp(0, 22, windowP),
		backdropAlpha: lerp(0, 0.38, windowP),
		openerOpacity,
		wordmarkOpacity,
		wordmarkBlur: lerp(14, 0, wordP),
		iconOpacity: welcomeP > 0.55 ? iconOpacity * (1 - welcomeP) : iconOpacity,
		skyBlur: lerp(44, 0, skyP),
		skyWash: lerp(0.5, 1, skyP),
		skySharpOpacity: lerp(0, 0.92, easeOut(Math.min(1, skyP * 1.15))),
		welcomeOpacity: welcomeP * 0.92,
		welcomeScale: lerp(0.96, 1, welcomeIn),
		shadowStrength: lerp(0, 0.42, windowP)
	};
}

export function introV2Complete(elapsed: number, reducedMotion: boolean): boolean {
	return elapsed >= (reducedMotion ? T.reducedTotal : T.total);
}
