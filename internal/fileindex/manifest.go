package fileindex

import "time"

const ManifestVersion = 1

type Manifest struct {
	Version int       `json:"version"`
	Kind    string    `json:"kind"`
	Archive string    `json:"archive"`
	Time    time.Time `json:"time"`
	BaseFull string   `json:"base_full"`
	Chain   []string  `json:"chain"`
	Deleted []string  `json:"deleted,omitempty"`
}

func NewFullManifest(archive string, t time.Time) Manifest {
	return Manifest{
		Version:  ManifestVersion,
		Kind:     KindFull,
		Archive:  archive,
		Time:     t.UTC(),
		BaseFull: archive,
		Chain:    []string{archive},
	}
}

func NewIncrementalManifest(archive string, t time.Time, baseFull string, chain []string, deleted []string) Manifest {
	cp := append([]string(nil), chain...)
	if cp == nil {
		cp = []string{}
	}
	del := append([]string(nil), deleted...)
	if del == nil {
		del = []string{}
	}
	return Manifest{
		Version:  ManifestVersion,
		Kind:     KindIncremental,
		Archive:  archive,
		Time:     t.UTC(),
		BaseFull: baseFull,
		Chain:    cp,
		Deleted:  del,
	}
}
