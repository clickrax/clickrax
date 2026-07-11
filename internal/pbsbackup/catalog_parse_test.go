package pbsbackup

import (
	"encoding/binary"
	"strings"
	"testing"
)

func appendU64_7bit(a []byte, v uint64) []byte {
	for {
		if v < 128 {
			return append(a, byte(v&0x7f))
		}
		a = append(a, byte(v&0x7f)|0x80)
		v >>= 7
	}
}

// buildTestCatalog mirrors pxar.go catalog layout: magic + main tree + root pointer + offset.
func buildTestCatalog() []byte {
	var out []byte
	out = append(out, catalogMagicBytes...)

	mainOld := len(out)
	table := make([]byte, 0)
	table = appendU64_7bit(table, 1) // one file
	table = append(table, 'f')
	table = appendU64_7bit(table, uint64(len("readme.txt")))
	table = append(table, []byte("readme.txt")...)
	table = appendU64_7bit(table, 42)
	table = appendU64_7bit(table, 1_700_000_000)
	out = appendU64_7bit(out, uint64(len(table)))
	out = append(out, table...)

	rootOld := len(out)
	rootTable := make([]byte, 0)
	rootTable = appendU64_7bit(rootTable, 1)
	rootTable = append(rootTable, 'd')
	rootTable = appendU64_7bit(rootTable, uint64(len("backup.pxar.didx")))
	rootTable = append(rootTable, []byte("backup.pxar.didx")...)
	rootTable = appendU64_7bit(rootTable, uint64(rootOld-mainOld))
	out = appendU64_7bit(out, uint64(len(rootTable)))
	out = append(out, rootTable...)

	out = binary.LittleEndian.AppendUint64(out, uint64(rootOld))
	return out
}

func TestParseCatalogAll(t *testing.T) {
	catalog := buildTestCatalog()
	files, err := parseCatalogAll(catalog)
	if err != nil {
		t.Fatalf("parseCatalogAll: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "readme.txt" {
		t.Fatalf("path: got %q", files[0].Path)
	}
	if files[0].Size != 42 {
		t.Fatalf("size: got %d", files[0].Size)
	}
}

func TestParseDirEntriesSymlinkDirect(t *testing.T) {
	catalog := buildNestedTestCatalog()
	contentStart, _, _, err := catalogContentRoot(catalog)
	if err != nil {
		t.Fatal(err)
	}
	subPos := findChildBlockStart(catalog, contentStart, "subdir")
	if subPos < 0 {
		t.Fatal("subdir block not found")
	}
	children, err := listCatalogDirChildren(catalog, subPos, "subdir", false)
	if err != nil {
		t.Fatalf("listCatalogDirChildren: %v", err)
	}
	if len(children) != 1 || children[0].Path != `subdir\a.txt` {
		t.Fatalf("got %+v", children)
	}
}

func findChildBlockStart(catalog []byte, blockStart int, name string) int {
	isRoot := false
	if contentStart, _, contentIsRoot, err := catalogContentRoot(catalog); err == nil && blockStart == contentStart {
		isRoot = contentIsRoot
	}
	children, err := listCatalogDirChildren(catalog, blockStart, "", isRoot)
	if err != nil {
		return -1
	}
	for _, child := range children {
		if child.IsDir && strings.EqualFold(catalogDirName(child.Path), name) {
			return child.dirBlockStart
		}
	}
	return -1
}

func catalogRootOffset(catalog []byte) int {
	pos, _ := catalogRootPos(catalog)
	return pos
}

func buildNestedTestCatalog() []byte {
	var out []byte
	out = append(out, catalogMagicBytes...)

	innerOld := len(out)
	inner := make([]byte, 0)
	inner = appendU64_7bit(inner, 1)
	inner = append(inner, 'f')
	inner = appendU64_7bit(inner, uint64(len("a.txt")))
	inner = append(inner, []byte("a.txt")...)
	inner = appendU64_7bit(inner, 1)
	inner = appendU64_7bit(inner, 1)
	out = appendU64_7bit(out, uint64(len(inner)))
	out = append(out, inner...)

	mainOld := len(out)
	main := make([]byte, 0)
	main = appendU64_7bit(main, 1)
	main = append(main, 'd')
	main = appendU64_7bit(main, uint64(len("subdir")))
	main = append(main, []byte("subdir")...)
	main = appendU64_7bit(main, uint64(mainOld-innerOld))
	out = appendU64_7bit(out, uint64(len(main)))
	out = append(out, main...)

	rootOld := len(out)
	root := make([]byte, 0)
	root = appendU64_7bit(root, 1)
	root = append(root, 'd')
	root = appendU64_7bit(root, uint64(len("backup.pxar.didx")))
	root = append(root, []byte("backup.pxar.didx")...)
	root = appendU64_7bit(root, uint64(rootOld-mainOld))
	out = appendU64_7bit(out, uint64(len(root)))
	out = append(out, root...)
	out = binary.LittleEndian.AppendUint64(out, uint64(rootOld))
	return out
}

func TestParseCatalogSymlinkNoPayload(t *testing.T) {
	var out []byte
	out = append(out, catalogMagicBytes...)

	mainOld := len(out)
	main := make([]byte, 0)
	main = appendU64_7bit(main, 2)
	main = append(main, 'l')
	main = appendU64_7bit(main, uint64(len("link.txt")))
	main = append(main, []byte("link.txt")...)
	main = append(main, 'f')
	main = appendU64_7bit(main, 0x10)
	main = append(main, []byte("1234567890123456")...)
	main = appendU64_7bit(main, 9)
	main = appendU64_7bit(main, 1)
	out = appendU64_7bit(out, uint64(len(main)))
	out = append(out, main...)

	rootOld := len(out)
	root := make([]byte, 0)
	root = appendU64_7bit(root, 1)
	root = append(root, 'd')
	root = appendU64_7bit(root, uint64(len("backup.pxar.didx")))
	root = append(root, []byte("backup.pxar.didx")...)
	root = appendU64_7bit(root, uint64(rootOld-mainOld))
	out = appendU64_7bit(out, uint64(len(root)))
	out = append(out, root...)
	out = binary.LittleEndian.AppendUint64(out, uint64(rootOld))

	files, err := parseCatalogAll(out)
	if err != nil {
		t.Fatalf("parseCatalogAll: %v", err)
	}
	if len(files) != 1 || files[0].Path != "1234567890123456" {
		t.Fatalf("unexpected files: %+v", files)
	}
}

func TestParseCatalogNestedDir(t *testing.T) {
	out := buildNestedTestCatalog()
	files, err := parseCatalogAll(out)
	if err != nil {
		t.Fatalf("parseCatalogAll: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 entries (dir+file), got %d: %+v", len(files), files)
	}
	var filePath string
	for _, f := range files {
		if !f.IsDir {
			filePath = f.Path
		}
	}
	if filePath != `subdir\a.txt` {
		t.Fatalf("nested path: got %q", filePath)
	}
}
