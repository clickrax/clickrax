package winattr

import "testing"

func TestEntryHashIncludesTimes(t *testing.T) {
	a := Entry{MtimeNS: 100}
	b := Entry{MtimeNS: 200}
	if a.Hash() == b.Hash() {
		t.Fatal("hash should change when mtime changes")
	}
}

func TestHasMetaTimesOnly(t *testing.T) {
	e := Entry{CtimeNS: 42}
	if !e.HasMeta() {
		t.Fatal("times-only entry should count as metadata")
	}
}
