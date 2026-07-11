package i18n

import (
	"errors"
	"fmt"
	"strings"

	"pbs-win-backup/internal/locale"
)

// Bundle provides localized strings for one language.
type Bundle struct {
	lang string
}

func New(lang string) *Bundle {
	return &Bundle{lang: locale.Normalize(lang)}
}

func (b *Bundle) Lang() string {
	if b == nil {
		return locale.Russian
	}
	return b.lang
}

func (b *Bundle) T(key string) string {
	return b.Tf(key, nil)
}

func (b *Bundle) Tf(key string, vars map[string]string) string {
	if b == nil {
		b = New(locale.Russian)
	}
	msg := lookup(b.lang, key)
	if msg == "" {
		if b.lang != locale.English {
			msg = lookup(locale.English, key)
		}
		if msg == "" {
			return key
		}
	}
	return interpolate(msg, vars)
}

func (b *Bundle) E(key string) error {
	return errors.New(b.T(key))
}

func (b *Bundle) Ef(key string, vars map[string]string) error {
	return errors.New(b.Tf(key, vars))
}

func (b *Bundle) Ewrap(key string, vars map[string]string, err error) error {
	if err == nil {
		return b.Ef(key, vars)
	}
	return fmt.Errorf("%s: %w", b.Tf(key, vars), err)
}

func interpolate(s string, vars map[string]string) string {
	if len(vars) == 0 {
		return s
	}
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

func lookup(lang, key string) string {
	if lang == locale.English {
		return en[key]
	}
	return ru[key]
}
