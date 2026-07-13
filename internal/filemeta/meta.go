package filemeta

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/winattr"
)

const Version = 1

const PBSBlobName = "backup.winmeta.blob"

// PBSBlobNameLegacy was used before v2.3.9; PBS rejects non-.blob extensions.
const PBSBlobNameLegacy = "backup.winmeta.json"

type Archive struct {
	Version int                    `json:"version"`
	Files   map[string]winattr.Entry `json:"files"`
}

func NewArchive() Archive {
	return Archive{Version: Version, Files: map[string]winattr.Entry{}}
}

func MetaFileName(archiveName string) string {
	return archiveName + ".meta.json"
}

func CatalogPath(rel string) string {
	return strings.ReplaceAll(rel, "/", `\`)
}

func CaptureFile(absPath string) (e winattr.Entry, err error) {
	defer func() {
		if r := recover(); r != nil {
			e = winattr.Entry{}
			err = fmt.Errorf("capture meta panic: %v", r)
		}
	}()
	return winattr.Capture(absPath)
}

func ApplyFile(destPath string, e winattr.Entry) error {
	if !e.HasMeta() {
		return nil
	}
	return winattr.Apply(destPath, e)
}

func (a *Archive) Set(catalogPath string, e winattr.Entry) {
	if a.Files == nil {
		a.Files = map[string]winattr.Entry{}
	}
	if !e.HasMeta() {
		return
	}
	a.Files[catalogPath] = e
}

func (a *Archive) Delete(catalogPath string) {
	if a.Files == nil {
		return
	}
	delete(a.Files, catalogPath)
}

func (a *Archive) MergeFrom(delta Archive, deleted []string) {
	if a.Files == nil {
		a.Files = map[string]winattr.Entry{}
	}
	for _, p := range deleted {
		delete(a.Files, p)
	}
	for path, e := range delta.Files {
		a.Files[path] = e
	}
}

func Marshal(a Archive) ([]byte, error) {
	if a.Files == nil {
		a.Files = map[string]winattr.Entry{}
	}
	a.Version = Version
	return json.MarshalIndent(a, "", "  ")
}

func Unmarshal(data []byte) (Archive, error) {
	var a Archive
	if err := json.Unmarshal(data, &a); err != nil {
		return a, err
	}
	if a.Files == nil {
		a.Files = map[string]winattr.Entry{}
	}
	return a, nil
}

func WalkTree(root string, skipAccess bool, onEntry func(catalogPath string, absPath string, isDir bool) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if skipAccess {
				return nil
			}
			return err
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = d.Name()
		}
		catalog := CatalogPath(filepath.ToSlash(rel))
		isDir := d.IsDir()
		if !isDir {
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
		}
		return onEntry(catalog, path, isDir)
	})
}

func CollectTree(root string, skipAccess bool) (Archive, error) {
	out := NewArchive()
	err := WalkTree(root, skipAccess, func(catalogPath, absPath string, _ bool) error {
		e, capErr := CaptureFile(absPath)
		if capErr != nil {
			if skipAccess {
				return nil
			}
			return capErr
		}
		out.Set(catalogPath, e)
		return nil
	})
	return out, err
}

func Lookup(a Archive, catalogPath string) (winattr.Entry, bool) {
	e, ok := a.Files[catalogPath]
	if ok {
		return e, true
	}
	lower := strings.ToLower(catalogPath)
	for k, v := range a.Files {
		if strings.ToLower(k) == lower {
			return v, true
		}
	}
	return winattr.Entry{}, false
}
