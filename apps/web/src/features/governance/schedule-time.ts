export function localScheduleTime(date: Date) {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
export function scheduleInstant(value: string, now = Date.now()) {
  const date = new Date(value);
  if (
    !Number.isFinite(date.getTime()) ||
    localScheduleTime(date) !== value ||
    date.getTime() <= now + 60000 ||
    date.getTime() > now + 365 * 86400000
  )
    return undefined;
  return date.toISOString();
}
