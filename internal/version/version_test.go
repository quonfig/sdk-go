package version

import (
	"strings"
	"testing"
)

func TestGet_NotEmpty(t *testing.T) {
	v := Get()
	if v == "" {
		t.Fatal("version.Get() returned empty string")
	}
	if strings.HasPrefix(v, "v") {
		t.Errorf("version.Get() should not include leading v: got %q", v)
	}
}

func TestHeader_HasGoPrefix(t *testing.T) {
	h := Header()
	if !strings.HasPrefix(h, "go-") {
		t.Errorf("version.Header() should start with go-: got %q", h)
	}
}

func TestTrimV(t *testing.T) {
	cases := map[string]string{
		"v1.2.3": "1.2.3",
		"1.2.3":  "1.2.3",
		"":       "",
		"vfoo":   "foo",
	}
	for in, want := range cases {
		if got := trimV(in); got != want {
			t.Errorf("trimV(%q) = %q, want %q", in, got, want)
		}
	}
}
