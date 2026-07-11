//go:build windows

package locale

import "syscall"

var procGetUserDefaultUILanguage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// SystemPreferred returns ru for Russian Windows UI, otherwise en.
func SystemPreferred() string {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	langID := uint16(r) & 0x3ff
	// LANG_RUSSIAN = 0x19
	if langID == 0x19 {
		return Russian
	}
	return English
}
