// Package i18n provides application-wide internationalization support for
// PWAMS. It supports English (en), Tamil (ta), and Sinhala (si) with English
// as the fallback language.
//
// Translation keys are stable internal identifiers; only display labels are
// translated. The T function looks up a key in the requested language and
// falls back to English when the key or language is missing.
package i18n

import (
	"strings"
)

// Supported language codes.
const (
	LangEnglish = "en"
	LangTamil   = "ta"
	LangSinhala = "si"
)

// DefaultLanguage is the fallback language used when a user preference is
// unset or unrecognised.
const DefaultLanguage = LangEnglish

// languageNames maps each supported language code to its native-display name.
var languageNames = map[string]string{
	LangEnglish: "English",
	LangTamil:   "தமிழ்",
	LangSinhala: "සිංහල",
}

// translations maps language code → key → label.
var translations = map[string]map[string]string{
	LangEnglish: english,
	LangTamil:   tamil,
	LangSinhala: sinhala,
}

// T returns the translated label for the given key in the requested language.
// If the language is unsupported or the key is missing, it falls back to the
// English translation. If the key is also missing from English, the key itself
// is returned so missing translations are visible in the UI.
func T(lang, key string) string {
	lang = normalizeLang(lang)

	if dict, ok := translations[lang]; ok {
		if val, ok := dict[key]; ok {
			return val
		}
	}

	// Fallback to English.
	if lang != DefaultLanguage {
		if dict, ok := translations[DefaultLanguage]; ok {
			if val, ok := dict[key]; ok {
				return val
			}
		}
	}

	// Last resort: return the key itself.
	return key
}

// SupportedLanguages returns a map of language code → native display name.
func SupportedLanguages() map[string]string {
	result := make(map[string]string, len(languageNames))
	for code, name := range languageNames {
		result[code] = name
	}
	return result
}

// IsSupportedLanguage reports whether the given language code is supported.
func IsSupportedLanguage(lang string) bool {
	lang = strings.ToLower(strings.TrimSpace(lang))
	_, ok := translations[lang]
	return ok
}

// normalizeLang lowercases and trims the language code, then validates it.
func normalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if _, ok := translations[lang]; ok {
		return lang
	}
	return DefaultLanguage
}

// EffectiveLanguage resolves the language to use for a request.
func EffectiveLanguage(userLang string) string {
	return normalizeLang(userLang)
}

// RoleKey converts an internal role identifier (stable, e.g. "Partner",
// "Super Admin") into the dictionary key suffix used by the role.* label
// family ("role.partner", "role.super_admin"). Authorization never uses the
// display label — this helper is purely for rendering localised labels.
func RoleKey(roleName string) string {
	role := strings.ToLower(strings.TrimSpace(roleName))
	switch role {
	case "super admin":
		return "super_admin"
	default:
		return strings.ReplaceAll(role, " ", "_")
	}
}

// Dictionary returns a copy of the translation dictionary for the requested
// language (falling back to English when the language is unsupported). It is
// used to ship the active-language dictionary to the client so JavaScript UI
// strings ({window.t}) stay localised without extra round trips.
func Dictionary(lang string) map[string]string {
	lang = normalizeLang(lang)
	dict, ok := translations[lang]
	if !ok {
		// normalizeLang always returns a supported language, but guard
		// anyway so Dictionary never returns nil.
		dict = translations[DefaultLanguage]
	}

	out := make(map[string]string, len(dict))
	for key, value := range dict {
		out[key] = value
	}
	return out
}
