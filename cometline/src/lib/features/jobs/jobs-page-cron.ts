export type ScheduleMode = 'one-shot' | 'recurring';
export type ScheduleFrequency = 'daily' | 'weekly' | 'monthly';

export type ParsedCronSchedule =
	| { frequency: 'daily'; time: string }
	| { frequency: 'weekly'; time: string; weekday: string }
	| { frequency: 'monthly'; time: string; monthDay: string };

export function localDatetimeToMillis(local: string): number | undefined {
	if (!local) return undefined;
	const ms = new Date(local).getTime();
	return Number.isNaN(ms) ? undefined : ms;
}

export function millisToLocalDatetime(ms?: number): string {
	if (!ms) return '';
	const date = new Date(ms);
	const offset = date.getTimezoneOffset() * 60_000;
	return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export function parseSupportedCron(expr: string): ParsedCronSchedule | null {
	const parts = expr.trim().split(/\s+/);
	if (parts.length !== 5) return null;
	const [minute, hour, dayOfMonth, month, dayOfWeek] = parts;
	if (month !== '*') return null;
	if (!/^\d+$/.test(minute) || !/^\d+$/.test(hour)) return null;
	const minuteNum = Number(minute);
	const hourNum = Number(hour);
	if (minuteNum < 0 || minuteNum > 59 || hourNum < 0 || hourNum > 23) return null;
	const time = `${String(hourNum).padStart(2, '0')}:${String(minuteNum).padStart(2, '0')}`;
	if (dayOfMonth === '*' && dayOfWeek === '*') {
		return { frequency: 'daily', time };
	}
	if (dayOfMonth === '*' && /^\d+$/.test(dayOfWeek)) {
		return {
			frequency: 'weekly',
			time,
			weekday: String(Math.min(6, Math.max(0, Number(dayOfWeek))))
		};
	}
	if (dayOfWeek === '*' && /^\d+$/.test(dayOfMonth)) {
		return {
			frequency: 'monthly',
			time,
			monthDay: String(Math.min(28, Math.max(1, Number(dayOfMonth))))
		};
	}
	return null;
}

export function buildCronExpression(
	time: string,
	frequency: ScheduleFrequency,
	weekday: string,
	monthDay: string
): string {
	const [hourRaw, minuteRaw] = time.split(':');
	const hour = Math.min(23, Math.max(0, Number(hourRaw) || 0));
	const minute = Math.min(59, Math.max(0, Number(minuteRaw) || 0));
	if (frequency === 'weekly') {
		return `${minute} ${hour} * * ${weekday}`;
	}
	if (frequency === 'monthly') {
		return `${minute} ${hour} ${monthDay} * *`;
	}
	return `${minute} ${hour} * * *`;
}

export function summarizeCronParts(
	frequency: ScheduleFrequency,
	time: string,
	weekday: string,
	monthDay: string
): string {
	const displayTime = time || '09:00';
	if (frequency === 'weekly') {
		const weekdays = [
			'Sunday',
			'Monday',
			'Tuesday',
			'Wednesday',
			'Thursday',
			'Friday',
			'Saturday'
		];
		return `Every ${weekdays[Number(weekday)] ?? 'Monday'} at ${displayTime}`;
	}
	if (frequency === 'monthly') {
		return `Every month on day ${monthDay} at ${displayTime}`;
	}
	return `Every day at ${displayTime}`;
}

export function cronDisplayLabel(expr: string): string {
	const parts = expr.trim().split(/\s+/);
	if (parts.length !== 5) return `cron: ${expr}`;
	const [minute, hour, dayOfMonth, month, dayOfWeek] = parts;
	if (month !== '*' || !/^\d+$/.test(minute) || !/^\d+$/.test(hour)) return `cron: ${expr}`;
	const time = `${String(Number(hour)).padStart(2, '0')}:${String(Number(minute)).padStart(2, '0')}`;
	if (dayOfMonth === '*' && dayOfWeek === '*') {
		return summarizeCronParts('daily', time, '1', '1');
	}
	if (dayOfMonth === '*' && /^\d+$/.test(dayOfWeek)) {
		return summarizeCronParts('weekly', time, dayOfWeek, '1');
	}
	if (dayOfWeek === '*' && /^\d+$/.test(dayOfMonth)) {
		return summarizeCronParts('monthly', time, '1', dayOfMonth);
	}
	return `cron: ${expr}`;
}
