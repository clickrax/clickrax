package locale

import "strings"

// Supported UI languages.
const (
	Russian = "ru"
	English = "en"
)

// Normalize maps a language tag to ru or en. Empty string uses system preference.
func Normalize(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch {
	case lang == Russian, strings.HasPrefix(lang, "ru-"):
		return Russian
	case lang == English, strings.HasPrefix(lang, "en-"):
		return English
	case lang == "":
		return SystemPreferred()
	default:
		return English
	}
}
