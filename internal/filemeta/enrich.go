package filemeta

import (
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/winattr"
)

// EnrichSnapshotFiles fills owner and attribute labels from archive metadata.
func EnrichSnapshotFiles(a Archive, files []models.SnapshotFile) []models.SnapshotFile {
	for i := range files {
		EnrichSnapshotFile(a, &files[i])
	}
	return files
}

// EnrichSnapshotFile updates a single catalog entry from metadata when present.
func EnrichSnapshotFile(a Archive, f *models.SnapshotFile) {
	if f == nil {
		return
	}
	e, ok := Lookup(a, f.Path)
	if !ok {
		return
	}
	if owner := winattr.OwnerLabel(e.SDDL); owner != "" {
		f.Owner = owner
	}
	if attrs := winattr.AttributesLabel(e.Attributes); attrs != "" {
		f.Attributes = attrs
	}
}
