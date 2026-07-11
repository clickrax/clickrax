package pbsbackup

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

var errCatalogSearchLimit = errors.New("catalog search limit reached")

type catalogDirRef struct {
	blockStart int
	prefix     string
	isRoot     bool
}

func catalogRootPos(catalog []byte) (int, error) {
	if len(catalog) < 16 {
		return 0, i18n.Ef("restore.catalog.empty_bytes", map[string]string{"n": fmt.Sprintf("%d", len(catalog))})
	}
	if !bytes.HasPrefix(catalog, catalogMagicBytes) {
		return 0, i18n.E("restore.catalog.bad_magic", nil)
	}
	return int(binary.LittleEndian.Uint64(catalog[len(catalog)-8:])), nil
}

func normalizeCatalogDirPath(dirPath string) string {
	dirPath = strings.TrimSpace(dirPath)
	dirPath = strings.Trim(dirPath, `/\`)
	return strings.ReplaceAll(dirPath, `/`, `\`)
}

func catalogDirName(path string) string {
	path = strings.TrimRight(path, `\`)
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func catalogContentRoot(catalog []byte) (blockStart int, prefix string, isRoot bool, err error) {
	rootPos, err := catalogRootPos(catalog)
	if err != nil {
		return 0, "", false, err
	}
	children, err := listCatalogDirChildren(catalog, rootPos, "", true)
	if err != nil {
		return 0, "", false, err
	}
	if len(children) == 1 && children[0].IsDir {
		return children[0].dirBlockStart, "", false, nil
	}
	return rootPos, "", true, nil
}

func resolveCatalogDir(catalog []byte, dirPath string) (catalogDirRef, error) {
	blockStart, prefix, isRoot, err := catalogContentRoot(catalog)
	if err != nil {
		return catalogDirRef{}, err
	}
	dirPath = normalizeCatalogDirPath(dirPath)
	ref := catalogDirRef{blockStart: blockStart, prefix: prefix, isRoot: isRoot}
	if dirPath == "" {
		return ref, nil
	}
	parts := strings.Split(dirPath, `\`)
	for _, part := range parts {
		if part == "" {
			continue
		}
		children, err := listCatalogDirChildren(catalog, ref.blockStart, ref.prefix, ref.isRoot)
		if err != nil {
			return catalogDirRef{}, err
		}
		found := false
		for _, child := range children {
			if child.IsDir && strings.EqualFold(catalogDirName(child.Path), part) {
				ref = catalogDirRef{
					blockStart: child.dirBlockStart,
					prefix:     child.Path,
					isRoot:     false,
				}
				found = true
				break
			}
		}
		if !found {
			return catalogDirRef{}, i18n.Ef("restore.catalog.not_found_dir", map[string]string{"path": dirPath})
		}
	}
	return ref, nil
}

type catalogDirChild struct {
	models.SnapshotFile
	dirBlockStart int
}

func listCatalogDirChildren(catalog []byte, blockStart int, prefix string, isRoot bool) ([]catalogDirChild, error) {
	if blockStart < 0 || blockStart >= len(catalog) {
		return nil, i18n.Ef("restore.catalog.pos_out_of_range", map[string]string{"pos": fmt.Sprintf("%d", blockStart)})
	}
	pos := blockStart
	tableLen, pos, err := readU64_7bit(catalog, pos)
	if err != nil {
		return nil, err
	}
	if pos+int(tableLen) > len(catalog) {
		return nil, i18n.E("restore.catalog.table_overflow", nil)
	}
	block := catalog[pos : pos+int(tableLen)]
	return parseCatalogDirChildren(catalog, block, blockStart, prefix, isRoot)
}

func parseCatalogDirChildren(catalog, block []byte, blockStart int, prefix string, isRoot bool) ([]catalogDirChild, error) {
	out := make([]catalogDirChild, 0, 32)
	pos := 0
	entryCount, pos, err := readU64_7bit(block, pos)
	if err != nil {
		return nil, err
	}

	var (
		nameLen uint64
		name    string
		rel     uint64
		size    uint64
		mtime   int64
	)

	for i := uint64(0); i < entryCount; i++ {
		if pos >= len(block) {
			return nil, i18n.Ef("restore.catalog.table_truncated", map[string]string{
				"i":     fmt.Sprintf("%d", i+1),
				"total": fmt.Sprintf("%d", entryCount),
			})
		}
		kind := block[pos]
		pos++
		switch kind {
		case 'd':
			nameLen, pos, err = readU64_7bit(block, pos)
			if err != nil {
				return nil, err
			}
			if pos+int(nameLen) > len(block) {
				return nil, i18n.E("restore.catalog.dir_name_truncated", nil)
			}
			name = string(block[pos : pos+int(nameLen)])
			pos += int(nameLen)
			rel, pos, err = readU64_7bit(block, pos)
			if err != nil {
				return nil, err
			}
			childStart := blockStart - int(rel)
			childPrefix := prefix
			if !isRoot {
				childPrefix = joinPath(prefix, name)
				out = append(out, catalogDirChild{
					SnapshotFile: models.SnapshotFile{
						Path:  childPrefix,
						IsDir: true,
					},
					dirBlockStart: childStart,
				})
			} else {
				out = append(out, catalogDirChild{
					SnapshotFile: models.SnapshotFile{
						Path:  name,
						IsDir: true,
					},
					dirBlockStart: childStart,
				})
			}
		case 'f':
			nameLen, pos, err = readU64_7bit(block, pos)
			if err != nil {
				return nil, err
			}
			if pos+int(nameLen) > len(block) {
				return nil, i18n.E("restore.catalog.file_name_truncated", nil)
			}
			name = string(block[pos : pos+int(nameLen)])
			pos += int(nameLen)
			size, pos, err = readU64_7bit(block, pos)
			if err != nil {
				return nil, err
			}
			mtime, pos, err = readI64_7bit(block, pos)
			if err != nil {
				return nil, err
			}
			full := joinPath(prefix, name)
			if isRoot {
				full = name
			}
			out = append(out, catalogDirChild{
				SnapshotFile: models.SnapshotFile{
					Path:     full,
					Size:     int64(size),
					IsDir:    false,
					Modified: time.Unix(mtime, 0).UTC().Format(time.RFC3339),
				},
			})
		case 'l', 'h', 'b', 'c', 'p', 's':
			nameLen, pos, err = readU64_7bit(block, pos)
			if err != nil {
				return nil, err
			}
			if pos+int(nameLen) > len(block) {
				return nil, i18n.E("restore.catalog.entry_name_truncated", nil)
			}
			pos += int(nameLen)
		default:
			return nil, i18n.Ef("restore.catalog.unknown_type", map[string]string{"type": string(kind)})
		}
	}
	if pos != len(block) {
		return nil, i18n.Ef("restore.catalog.table_unparsed", map[string]string{"n": fmt.Sprintf("%d", len(block)-pos)})
	}
	return out, nil
}

func listCatalogDirEntries(catalog []byte, dirPath string) ([]models.SnapshotFile, error) {
	ref, err := resolveCatalogDir(catalog, dirPath)
	if err != nil {
		return nil, err
	}
	children, err := listCatalogDirChildren(catalog, ref.blockStart, ref.prefix, ref.isRoot)
	if err != nil {
		return nil, err
	}
	out := make([]models.SnapshotFile, 0, len(children))
	for _, child := range children {
		out = append(out, child.SnapshotFile)
	}
	if len(out) == 0 {
		return out, nil
	}
	return out, nil
}

func forEachCatalogFile(catalog []byte, fn func(models.SnapshotFile) error) error {
	blockStart, prefix, isRoot, err := catalogContentRoot(catalog)
	if err != nil {
		return err
	}
	return walkCatalogDir(catalog, blockStart, prefix, isRoot, fn)
}

func walkCatalogDir(catalog []byte, blockStart int, prefix string, isRoot bool, fn func(models.SnapshotFile) error) error {
	children, err := listCatalogDirChildren(catalog, blockStart, prefix, isRoot)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := fn(child.SnapshotFile); err != nil {
			return err
		}
		if child.IsDir {
			if err := walkCatalogDir(catalog, child.dirBlockStart, child.Path, false, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func searchCatalogFiles(catalog []byte, query string, limit int) ([]models.SnapshotFile, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, i18n.E("restore.catalog.search_empty", nil)
	}
	if limit <= 0 {
		limit = 500
	}
	out := make([]models.SnapshotFile, 0, catalogSearchLimitCap(limit))
	err := forEachCatalogFile(catalog, func(f models.SnapshotFile) error {
		if strings.Contains(strings.ToLower(f.Path), q) {
			out = append(out, f)
			if len(out) >= limit {
				return errCatalogSearchLimit
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errCatalogSearchLimit) {
		return nil, err
	}
	return out, nil
}

func catalogSearchLimitCap(limit int) int {
	if limit < 64 {
		return limit
	}
	return 64
}

func readU64_7bit(data []byte, pos int) (uint64, int, error) {
	if pos >= len(data) {
		return 0, pos, fmt.Errorf("unexpected EOF")
	}
	var v uint64
	shift := uint(0)
	for pos < len(data) {
		b := data[pos]
		pos++
		v |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return v, pos, nil
		}
		shift += 7
		if shift > 63 {
			return 0, pos, i18n.E("restore.catalog.bit7_overflow", nil)
		}
	}
	return 0, pos, i18n.E("restore.catalog.bit7_incomplete", nil)
}

// readI64_7bit matches catalog_decode_i64 in proxmox-backup (pbs-datastore).
func readI64_7bit(data []byte, pos int) (int64, int, error) {
	var v uint64
	for i := 0; i < 11; i++ {
		if pos >= len(data) {
			return 0, pos, fmt.Errorf("unexpected EOF")
		}
		b := data[pos]
		pos++
		switch {
		case b == 0:
			if v == 0 {
				return 0, pos, nil
			}
			return ((int64(v) - 1) * -1) - 1, pos, nil
		case b < 128:
			v |= uint64(b) << (i * 7)
			return int64(v), pos, nil
		default:
			v |= uint64(b&127) << (i * 7)
		}
	}
	return 0, pos, i18n.E("restore.catalog.i64_incomplete", nil)
}

func parseCatalogAll(catalog []byte) ([]models.SnapshotFile, error) {
	if _, err := catalogRootPos(catalog); err != nil {
		return nil, err
	}
	files := make([]models.SnapshotFile, 0)
	err := forEachCatalogFile(catalog, func(f models.SnapshotFile) error {
		files = append(files, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, i18n.Ef("restore.catalog.empty", map[string]string{"n": fmt.Sprintf("%d", len(catalog))})
	}
	return files, err
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return strings.TrimRight(prefix, `/\`) + `\` + name
}

func filterFiles(files []models.SnapshotFile, query string) []models.SnapshotFile {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return files
	}
	out := make([]models.SnapshotFile, 0)
	for _, f := range files {
		if strings.Contains(strings.ToLower(f.Path), q) {
			out = append(out, f)
		}
	}
	return out
}
