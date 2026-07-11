package winattr

import "testing"

func TestACLHashIgnoresTimestamps(t *testing.T) {
	a := Entry{SDDL: "O:BA", Attributes: 0x20, MtimeNS: 100, AtimeNS: 200}
	b := Entry{SDDL: "O:BA", Attributes: 0x20, MtimeNS: 999, AtimeNS: 888}
	if ACLHash(a) != ACLHash(b) {
		t.Fatal("ACLHash must ignore timestamps")
	}
}

func TestACLHashChangesOnSDDL(t *testing.T) {
	a := Entry{SDDL: "O:BA", Attributes: 0x20}
	b := Entry{SDDL: "O:BU", Attributes: 0x20}
	if ACLHash(a) == ACLHash(b) {
		t.Fatal("ACLHash must reflect SDDL changes")
	}
}
