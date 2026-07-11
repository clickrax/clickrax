package credential

import (
	"testing"
)

func TestValidateCredentialIDRejectsTraversal(t *testing.T) {
	if err := validateCredentialID(`..\..\windows`); err == nil {
		t.Fatal("path traversal id should be rejected")
	}
	if err := validateCredentialID("not-a-uuid"); err == nil {
		t.Fatal("non-uuid should be rejected")
	}
}
