/**
 * Offline runtime localisation helper.
 *
 * The server injects the active-language dictionary as `window.PWAMS_I18N`
 * (the same convention used by other PWAMS front-end scripts). Offline
 * modules must never hardcode user-facing strings: they resolve the key
 * through this helper and fall back to the canonical English wording when
 * the key (or the whole dictionary) is unavailable.
 */
declare global {
  interface Window {
    PWAMS_I18N?: Record<string, string>;
  }
}

export function localize(key: string, fallback: string): string {
  if (typeof window === "undefined") return fallback;
  const translated = window.PWAMS_I18N?.[key];
  return typeof translated === "string" && translated.length > 0
    ? translated
    : fallback;
}
