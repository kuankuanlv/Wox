package launcher

import (
	"os"
	"path/filepath"
	"strings"
)

// isPathInput reports whether text looks like an absolute path. Relative paths
// (foo/bar) are intentionally not treated as path input to avoid clashing with
// keyword queries. A leading "~" never reaches here: the input layer expands it
// to the home directory before any query text is consumed.
func isPathInput(text string) bool {
	if text == "" {
		return false
	}
	return strings.HasPrefix(text, "/")
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
	var dir string
	var base string
	if strings.HasSuffix(text, "/") {
		dir = text
		base = ""
	} else {
		dir = filepath.Dir(text)
		base = filepath.Base(text)
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
		return completed, true
	}
	return "", false
}
