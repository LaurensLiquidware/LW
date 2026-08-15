/**
 * Formats a number using the given locale's actual conventions (decimal
 * comma vs. period, grouping) — Angular's `number` pipe uses the app's
 * static LOCALE_ID (fixed at bootstrap to en-US), which never reflects a
 * runtime Transloco language switch. Using Intl directly, keyed off the
 * currently active language, keeps figures correctly regional per
 * project brief §11 instead of silently staying US-formatted.
 */
export function formatLocaleNumber(value: number, locale: string, fractionDigits: number): string {
  return new Intl.NumberFormat(locale, { minimumFractionDigits: fractionDigits, maximumFractionDigits: fractionDigits }).format(value);
}
