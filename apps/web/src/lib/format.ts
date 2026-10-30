export const exampleInstant = "2026-01-15T04:30:00Z";
export function formatDate(
  value: string,
  locale: string,
  timeZone = "Asia/Shanghai",
) {
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "long",
    timeStyle: "short",
    timeZone,
  }).format(new Date(value));
}
export function formatNumber(value: number, locale: string) {
  return new Intl.NumberFormat(locale).format(value);
}
export function formatCurrency(
  value: number,
  locale: string,
  currency = "USD",
) {
  return new Intl.NumberFormat(locale, { style: "currency", currency }).format(
    value,
  );
}
