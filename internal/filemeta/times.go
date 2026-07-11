package filemeta

import (
	"time"

	"pbs-win-backup/internal/winattr"
)

// MergeModifiedFallback fills modification time from catalog when meta has no stored mtime.
func MergeModifiedFallback(e *winattr.Entry, modifiedRFC3339 string) {
	if e == nil || e.MtimeNS != 0 || modifiedRFC3339 == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, modifiedRFC3339)
	if err != nil {
		return
	}
	e.MtimeNS = t.UTC().UnixNano()
}

// PrepareEntry returns stored metadata with optional catalog mtime fallback.
func PrepareEntry(a Archive, catalogPath, modifiedRFC3339 string) winattr.Entry {
	e, ok := Lookup(a, catalogPath)
	if !ok {
		e = winattr.Entry{}
	}
	MergeModifiedFallback(&e, modifiedRFC3339)
	return e
}
