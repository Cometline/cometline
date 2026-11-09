import { clamp01, easeOut, seg } from './intro-timeline';
import { INTRO_V2_BEATS as T } from './intro-timeline-v2';

export type IntroV2Motion = {
	windowScale: number;
	windowRadius: number;
	backdropAlpha: number;
	openerOpacity: number;
	cometOpacity: number;
	cometLift: number;
	wordmarkOpacity: number;
	wordmarkBlur: number;
	skyBlur: number;
	skyWash: number;
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
			cometOpacity: 0,
			cometLift: 0,
			wordmarkOpacity: Math.min(1, t * 2) * (1 - t),
			wordmarkBlur: 0,
			skyBlur: lerp(12, 0, easeOut(t)),
			skyWash: 1,
			welcomeOpacity: t > 0.55 ? easeOut((t - 0.55) / 0.45) : 0,
			welcomeScale: 1,
			shadowStrength: 0.35
		};
	}

	const windowP = easeOut(seg(elapsed, 0, T.windowSettle));
	const openerFade = 1 - easeOut(seg(elapsed, T.openerHold, T.openerOut));
	const cometP = easeOut(seg(elapsed, T.openerOut - 200, T.cometRise));
	const wordP = easeOut(seg(elapsed, T.wordmarkIn - 400, T.wordmarkIn + 900));
	const brandOut = easeOut(seg(elapsed, T.brandHold, T.brandOut));
	const brandOpacity = Math.max(0, 1 - brandOut);
	const skyP = easeOut(seg(elapsed, T.brandOut - 200, T.skyReveal));
	const welcomeP = easeOut(seg(elapsed, T.welcomeIn, T.welcomeIn + 1200));

	return {
		windowScale: lerp(1, 0.9, windowP),
		windowRadius: lerp(0, 22, windowP),
		backdropAlpha: lerp(0, 0.38, windowP),
		openerOpacity: openerFade,
		cometOpacity: brandOpacity * Math.min(1, cometP * 1.4),
		cometLift: cometP,
		wordmarkOpacity: brandOpacity * wordP,
		wordmarkBlur: lerp(10, 0, wordP),
		skyBlur: lerp(36, 0, skyP),
		skyWash: lerp(0.35, 1, skyP),
		welcomeOpacity: welcomeP,
		welcomeScale: lerp(0.96, 1, welcomeP),
		shadowStrength: lerp(0, 0.42, windowP)
	};
}

export function introV2Complete(elapsed: number, reducedMotion: boolean): boolean {
	return elapsed >= (reducedMotion ? T.reducedTotal : T.total);
}
