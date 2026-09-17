package i18n

import (
	"embed"
	"encoding/json"
)

//go:embed locales/*.json
var localeFS embed.FS

var translations = map[string]map[string]string{}

func init() {
	for _, lang := range []string{"en", "ta", "si"} {
		data, err := localeFS.ReadFile("locales/" + lang + ".json")
		if err != nil {
			panic("i18n: failed to load locale " + lang + ": " + err.Error())
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			panic("i18n: failed to parse locale " + lang + ": " + err.Error())
		}
		translations[lang] = m
	}
}

// SupportedLanguages lists the languages PWAMS ships with, per SRS
// section 32 (English, Tamil, Sinhala).
var SupportedLanguages = []string{"en", "ta", "si"}

func IsSupported(lang string) bool {
	_, ok := translations[lang]
	return ok
}

// T returns the translation for key in lang, falling back to English,
// then to the raw key, so a missing translation never breaks a page.
func T(lang, key string) string {
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
	}
	if v, ok := translations["en"][key]; ok && v != "" {
		return v
	}
	return key
}
