package pbsbackup

import "testing"

func TestListCatalogDirEntriesRoot(t *testing.T) {
	catalog := buildTestCatalog()
	root, err := listCatalogDirEntries(catalog, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(root) == 0 {
		t.Fatal("expected root children")
	}
}

func TestListCatalogDirEntriesNested(t *testing.T) {
	catalog := buildNestedTestCatalog()
	children, err := listCatalogDirEntries(catalog, "subdir")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Path != `subdir\a.txt` {
		t.Fatalf("got %+v", children)
	}
}

func TestSearchCatalogFilesLimit(t *testing.T) {
	catalog := buildNestedTestCatalog()
	out, err := searchCatalogFiles(catalog, "a.txt", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1 result, got %d", len(out))
	}
}
