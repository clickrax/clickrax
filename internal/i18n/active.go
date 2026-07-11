package i18n

import "sync/atomic"

var active atomic.Pointer[Bundle]

// SetActive sets the bundle used by L/E helpers during backup/restore runs.
func SetActive(b *Bundle) {
	if b == nil {
		b = New("")
	}
	active.Store(b)
}

// Active returns the current run bundle or loads language from config.
func Active() *Bundle {
	if b := active.Load(); b != nil {
		return b
	}
	return New("")
}

// RunWith executes fn with b as the active localization bundle.
func RunWith(b *Bundle, fn func()) {
	prev := active.Load()
	SetActive(b)
	defer func() {
		if prev != nil {
			active.Store(prev)
		} else {
			active.Store(nil)
		}
	}()
	fn()
}

// L translates key using the active bundle.
func L(key string, vars map[string]string) string {
	return Active().Tf(key, vars)
}

// E returns an error with a localized message.
func E(key string, vars map[string]string) error {
	return Ef(key, vars)
}

// Ef returns an error with a localized formatted message.
func Ef(key string, vars map[string]string) error {
	return Active().Ef(key, vars)
}

// Ewrap returns a wrapped error with a localized prefix.
func Ewrap(key string, vars map[string]string, err error) error {
	return Active().Ewrap(key, vars, err)
}
