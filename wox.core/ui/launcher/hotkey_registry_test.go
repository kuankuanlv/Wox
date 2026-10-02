package launcher

import (
	"runtime"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
)

func TestBuiltinHotkeyDefinitions(t *testing.T) {
	definitions := builtinHotkeyDefinitions()
	if len(definitions) != 4 {
		t.Fatalf("built-in hotkey registry = %d entries, want 4 (about/filter/attention/settings)", len(definitions))
	}
	seen := map[string]bool{}
	for _, def := range definitions {
		if seen[def.ID] {
			t.Fatalf("duplicate built-in hotkey id %q", def.ID)
		}
		seen[def.ID] = true
		if strings.TrimSpace(def.DefaultKey) == "" || strings.TrimSpace(def.AltKey) == "" {
			t.Fatalf("built-in hotkey %q has an empty default key", def.ID)
		}
		if strings.HasPrefix(def.DefaultKey, "command+") == (strings.HasPrefix(def.AltKey, "command+")) {
			t.Fatalf("built-in hotkey %q: darwin and alt defaults must differ in primary modifier", def.ID)
		}
	}
}

func TestBuiltinHotkeyDefaultKeys(t *testing.T) {
	wantDarwin := map[string]string{
		builtinHotkeyAbout:     "command+shift+k",
		builtinHotkeyFilter:    "command+f",
		builtinHotkeyAttention: "command+u",
		builtinHotkeySettings:  "command+,",
	}
	wantOther := map[string]string{
		builtinHotkeyAbout:     "control+shift+k",
		builtinHotkeyFilter:    "control+f",
		builtinHotkeyAttention: "control+u",
		builtinHotkeySettings:  "control+,",
	}
	want := wantOther
	if runtime.GOOS == "darwin" {
		want = wantDarwin
	}
	for id, expected := range want {
		if got := builtinHotkeyDefaultKey(id); got != expected {
			t.Errorf("builtinHotkeyDefaultKey(%q) = %q, want %q", id, got, expected)
		}
	}
}

func TestBuiltinHotkeyOverrideAndFallback(t *testing.T) {
	if got := builtinHotkeyFor(builtinHotkeySettings, nil); got != builtinHotkeyDefaultKey(builtinHotkeySettings) {
		t.Fatalf("nil overrides should fall back to default, got %q", got)
	}
	overrides := map[string]string{builtinHotkeySettings: "command+option+s"}
	if got := builtinHotkeyFor(builtinHotkeySettings, overrides); got != "command+option+s" {
		t.Fatalf("override should win, got %q", got)
	}
	overrides[builtinHotkeySettings] = "   "
	if got := builtinHotkeyFor(builtinHotkeySettings, overrides); got != builtinHotkeyDefaultKey(builtinHotkeySettings) {
		t.Fatalf("blank override should fall back to default, got %q", got)
	}
	if got := builtinHotkeyFor(builtinHotkeyFilter, overrides); got != builtinHotkeyDefaultKey(builtinHotkeyFilter) {
		t.Fatalf("missing override should fall back to default, got %q", got)
	}
}

func TestMatchesBuiltinHotkey(t *testing.T) {
	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	overrides := map[string]string{builtinHotkeyFilter: "command+g"}
	if runtime.GOOS != "darwin" {
		overrides[builtinHotkeyFilter] = "control+g"
	}
	if !matchesBuiltinHotkey(builtinHotkeyFilter, overrides, woxui.KeyEvent{Key: "g", Modifiers: primary, Down: true}) {
		t.Fatal("custom filter hotkey should match the overridden key")
	}
	if matchesBuiltinHotkey(builtinHotkeyFilter, overrides, woxui.KeyEvent{Key: "f", Modifiers: primary, Down: true}) {
		t.Fatal("old default key must stop matching after override")
	}
	if matchesBuiltinHotkey(builtinHotkeyFilter, overrides, woxui.KeyEvent{Key: "g", Modifiers: primary, Down: true, Composing: true}) {
		t.Fatal("IME composition must not match built-in hotkeys")
	}
	if matchesBuiltinHotkey(builtinHotkeyFilter, nil, woxui.KeyEvent{Key: "g", Modifiers: primary, Down: true}) {
		t.Fatal("no override: wrong key must not match the default")
	}
}

func TestBuiltinHotkeySettingsCommaKey(t *testing.T) {
	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	if !matchesBuiltinHotkey(builtinHotkeySettings, nil, woxui.KeyEvent{Key: ",", Modifiers: primary, Down: true}) {
		t.Fatalf("settings hotkey %q should match comma+primary", builtinHotkeyDefaultKey(builtinHotkeySettings))
	}
}
