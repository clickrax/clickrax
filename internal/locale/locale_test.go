package locale

import "testing"

func TestNormalize_explicit(t *testing.T) {
	if Normalize("ru") != Russian {
		t.Fatal("ru")
	}
	if Normalize("en-US") != English {
		t.Fatal("en-US")
	}
	if Normalize("de") != English {
		t.Fatal("de -> en")
	}
}
