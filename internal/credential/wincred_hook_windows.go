//go:build windows

package credential

import "github.com/danieljoos/wincred"

var writeGenericCredential = func(c *wincred.GenericCredential) error {
	return c.Write()
}
