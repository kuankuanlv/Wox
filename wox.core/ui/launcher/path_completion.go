package launcher

import (
	"os"
	"path/filepath"
	"strings"
)

// isPathInput reports whether text looks like an absolute path or a "~/" path.
// Relative paths (foo/bar) are intentionally not treated as path input to avoid
// clashing with keyword queries.
func isPathInput(text string) bool {
	if text == "" {
		return false
	}
	return strings.HasPrefix(text, "/") || strings.HasPrefix(text, "~/")
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

// firstPathChildCompletion expands "~", lists the parent directory and returns
// the completed text for the first child whose name starts with the typed base.
// The leading "~" shorthand is preserved when the input used it. Matching is
// case-insensitive (macOS default filesystem behaviour); hidden children are
// matched only when the typed base itself starts with ".".
func firstPathChildCompletion(text string) (string, bool) {
	displayPrefix := text
	absolutePrefix := text
	if strings.HasPrefix(text, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		absolutePrefix = filepath.Join(home, text[2:])
	}

	var dir string
	var base string
	if strings.HasSuffix(absolutePrefix, "/") {
		dir = absolutePrefix
		base = ""
	} else {
		dir = filepath.Dir(absolutePrefix)
		base = filepath.Base(absolutePrefix)
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
		display := name
		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			// Follow symlinks (e.g. /tmp -> /private/tmp on macOS) so a link
			// to a directory completes with a trailing "/" and the next Tab
			// can descend into it.
			if info, err := os.Stat(filepath.Join(dir, name)); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			display += "/"
		}
		return displayPrefix + display[len(base):], true
	}
	return "", false
}
