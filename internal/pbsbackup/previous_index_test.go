package pbsbackup

import (
	"errors"
	"testing"
)

func TestPreviousIndexUnavailable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("PBS backup upgrade HTTP 400 Bad Request"), true},
		{errors.New("previous HTTP 400: no valid previous backup"), true},
		{errors.New("connection reset"), false},
	}
	for _, tc := range cases {
		if got := previousIndexUnavailable(tc.err); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.err, got, tc.want)
		}
	}
}
