package launcher

import (
	"os"
	"path/filepath"
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

// setupFakeHome points $HOME at a temp directory holding a hidden directory
// ".agents" and a plain file "notes.txt", mirroring the real ~/.agents case.
func setupFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, ".agents"), 0755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "notes.txt"), []byte("notes"), 0644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}
	return home
}

func TestExpandQueryTextHomeBareTildeGetsTrailingSlash(t *testing.T) {
	home := setupFakeHome(t)
	got, ok := expandQueryTextHome("~")
	if !ok {
		t.Fatal("expandQueryTextHome(~) reported no expansion")
	}
	if want := home + "/"; got != want {
		t.Fatalf("expandQueryTextHome(~) = %q, want %q", got, want)
	}
}

func TestExpandQueryTextHomeTildePrefix(t *testing.T) {
	home := setupFakeHome(t)
	got, ok := expandQueryTextHome("~/.agen")
	if !ok {
		t.Fatal("expandQueryTextHome(~/...) reported no expansion")
	}
	if want := home + "/.agen"; got != want {
		t.Fatalf("expandQueryTextHome(~/...) = %q, want %q", got, want)
	}
}

func TestExpandQueryTextHomeLeavesAbsoluteAlone(t *testing.T) {
	setupFakeHome(t)
	if got, ok := expandQueryTextHome("/usr/bin/foo"); ok || got != "/usr/bin/foo" {
		t.Fatalf("absolute path = %q/%v, want unchanged", got, ok)
	}
}

func TestTildeDirectoryCompletionKeepsTildeAndTrailingSlash(t *testing.T) {
	setupFakeHome(t)
	got, ok := firstPathChildCompletion("~/.agen")
	if !ok {
		t.Fatal("firstPathChildCompletion(~/.agen) = no match")
	}
	if want := "~/.agents/"; got != want {
		t.Fatalf("~/.agen Tab = %q, want %q", got, want)
	}
}

func TestTildeFileCompletionHasNoTrailingSlash(t *testing.T) {
	setupFakeHome(t)
	got, ok := firstPathChildCompletion("~/notes")
	if !ok {
		t.Fatal("firstPathChildCompletion(~/notes) = no match")
	}
	if want := "~/notes.txt"; got != want {
		t.Fatalf("~/notes Tab = %q, want %q", got, want)
	}
	if strings.HasSuffix(got, "/") {
		t.Fatalf("file completion %q must not end with slash", got)
	}
}

func TestAbsoluteDirectoryCompletionKeepsTrailingSlash(t *testing.T) {
	home := setupFakeHome(t)
	got, ok := firstPathChildCompletion(filepath.Join(home, ".agen"))
	if !ok {
		t.Fatal("absolute .agen completion = no match")
	}
	if want := filepath.Join(home, ".agents") + "/"; got != want {
		t.Fatalf("absolute .agen Tab = %q, want %q", got, want)
	}
}
