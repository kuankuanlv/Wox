package launcher

import (
	"strings"
	"testing"
)

func TestIsPathInput(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"", false},
		{"foo", false},
		{"foo/bar", false},
		{"~/.agents", true},
		{"~/", true},
		{"/Users", true},
		{"/", true},
		{"gongji", false},
	}
	for _, c := range cases {
		if got := isPathInput(c.text); got != c.want {
			t.Errorf("isPathInput(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestFirstPathChildCompletion(t *testing.T) {
	// /tmp always exists and contains at least one entry; the completion must
	// resolve to a child under /tmp/ (file or directory).
	got, ok := firstPathChildCompletion("/tmp/")
	if !ok || !strings.HasPrefix(got, "/tmp/") {
		t.Fatalf(`firstPathChildCompletion("/tmp/") = %q, %v; want a child under /tmp/`, got, ok)
	}

	// A missing prefix must fall back cleanly.
	if _, ok := firstPathChildCompletion("/definitely-not-a-real-path-xyz/"); ok {
		t.Fatalf("expected fallback for a nonexistent path")
	}

	// Base-prefix completion preserves the typed prefix and adds the separator.
	got2, ok2 := firstPathChildCompletion("/tm")
	if !ok2 || got2 != "/tmp/" {
		t.Fatalf(`firstPathChildCompletion("/tm") = %q, %v; want "/tmp/"`, got2, ok2)
	}
}
