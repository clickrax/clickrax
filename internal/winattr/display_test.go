package winattr

import "testing"

func TestOwnerLabelWellKnown(t *testing.T) {
	sddl := "O:BA G:BU D:(A;OICI;FA;;;SY)"
	if got := OwnerLabel(sddl); got != "BUILTIN\\Administrators" {
		t.Fatalf("OwnerLabel() = %q, want BUILTIN\\Administrators", got)
	}
}

func TestOwnerSIDFromSDDL(t *testing.T) {
	sid := ownerSIDFromSDDL("O:S-1-5-21-1 G:BU D:AI")
	if sid != "S-1-5-21-1" {
		t.Fatalf("ownerSIDFromSDDL() = %q", sid)
	}
}

func TestAttributesLabel(t *testing.T) {
	got := AttributesLabel(0x21) // readonly + archive
	if got != "R A" {
		t.Fatalf("AttributesLabel() = %q, want R A", got)
	}
}
