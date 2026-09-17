export function localize(key, fallback) {
    if (typeof window === "undefined")
        return fallback;
    const translated = window.PWAMS_I18N?.[key];
    return typeof translated === "string" && translated.length > 0
        ? translated
        : fallback;
}
//# sourceMappingURL=i18n.js.map