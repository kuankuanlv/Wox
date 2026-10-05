package launcher

import (
	"os"
	"path/filepath"
	"strings"
)

// isPathInput reports whether text looks like a path. Relative paths
// (foo/bar) are intentionally not treated as path input to avoid clashing with
// keyword queries. A leading "~" (bare home or "~/…") is treated as path input
// so Tab completion can expand it; the input layer additionally rewrites "~"
// to the absolute home directory while typing.
func isPathInput(text string) bool {
	if text == "" {
		return false
	}
	return strings.HasPrefix(text, "/") || text == "~" || strings.HasPrefix(text, "~/")
}

// expandTildeForLookup maps a leading "~" to the absolute home directory for
// filesystem access. It returns the lookup path and whether tilde was used, so
// the caller can rewrite the result back into the user's tilde spelling.
func expandTildeForLookup(text string) (lookup string, usedTilde bool, home string) {
	if text == "~" {
		if dir, err := os.UserHomeDir(); err == nil {
			return dir, true, dir
		}
		return text, false, ""
	}
	if strings.HasPrefix(text, "~/") {
		if dir, err := os.UserHomeDir(); err == nil {
			return dir + text[1:], true, dir
		}
	}
	return text, false, ""
}

// tildePrefix rewrites an absolute path back into the user's tilde spelling when
// the original input used "~". The trailing slash is preserved for directories.
func tildePrefix(path, home string) string {
	if home == "" || path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + rest
	}
	return path
}

// completePathTab completes the current editor text to the first child that
// matches the typed path prefix. Directories gain a trailing "/" so the next
// Tab continues one level deeper; files complete without a separator. It
// returns false when the input is not a path or no child matches, letting the
// caller fall back to the default Tab behaviour.
func (a *App) completePathTab() bool {
	text := a.editor.State().Text
	if !isPathInput(text) {
		return false
	}
	completion, ok := firstPathChildCompletion(text)
	if !ok {
		return false
	}
	a.rememberQueryHint()
	a.editor.SetText(completion, false)
	a.applyQueryTextChangeLocked(completion)
	a.reconcileSelectedPreview()
	_ = a.window.Invalidate()
	if err := a.sendCurrentQuery(); err != nil {
		// Keep the completion even if the refresh fails; the next query will
		// pick it up.
	}
	return true
}

// firstPathChildCompletion lists the parent directory and returns the completed
// text for the first child whose name starts with the typed base. Matching is
// case-insensitive (macOS default filesystem behaviour); hidden children are
// matched only when the typed base itself starts with ".".
func firstPathChildCompletion(text string) (string, bool) {
	lookup, usedTilde, home := expandTildeForLookup(text)

	var dir string
	var base string
	if strings.HasSuffix(lookup, "/") {
		dir = lookup
		base = ""
	} else {
		dir = filepath.Dir(lookup)
		base = filepath.Base(lookup)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	lowerBase := strings.ToLower(base)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(strings.ToLower(name), lowerBase) {
			continue
		}
		if !strings.HasPrefix(base, ".") && strings.HasPrefix(name, ".") {
			continue
		}
		completed := filepath.Join(dir, name)
		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			// Follow symlinks (e.g. /tmp -> /private/tmp on macOS) so a link
			// to a directory completes with a trailing "/" and the next Tab
			// can descend into it.
			if info, err := os.Stat(completed); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			completed += "/"
		}
		if usedTilde {
			completed = tildePrefix(completed, home)
		}
		return completed, true
	}
	return "", false
}
