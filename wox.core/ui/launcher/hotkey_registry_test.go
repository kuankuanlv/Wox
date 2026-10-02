package launcher

import (
	"runtime"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
)

func TestBuiltinHotkeyDefinitions(t *testing.T) {
	definitions := builtinHotkeyDefinitions()
	if len(definitions) != 3 {
		t.Fatalf("built-in hotkey registry = %d entries, want 3 (about/actionPanel/settings)", len(definitions))
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
		builtinHotkeyAbout:       "command+shift+k",
		builtinHotkeyActionPanel: "command+k",
		builtinHotkeySettings:    "command+,",
	}
	wantOther := map[string]string{
		builtinHotkeyAbout:       "control+shift+k",
		builtinHotkeyActionPanel: "control+k",
		builtinHotkeySettings:    "control+,",
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
}

func TestMatchesBuiltinHotkeyOverrideAndIME(t *testing.T) {
	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	overrides := map[string]string{builtinHotkeyActionPanel: "command+o"}
	if runtime.GOOS != "darwin" {
		overrides[builtinHotkeyActionPanel] = "control+o"
	}
	if !matchesBuiltinHotkey(builtinHotkeyActionPanel, overrides, woxui.KeyEvent{Key: "o", Modifiers: primary, Down: true}) {
		t.Fatal("custom actionPanel hotkey should match the overridden key")
	}
	if matchesBuiltinHotkey(builtinHotkeyActionPanel, overrides, woxui.KeyEvent{Key: "k", Modifiers: primary, Down: true}) {
		t.Fatal("old default key must stop matching after override")
	}
	if matchesBuiltinHotkey(builtinHotkeyActionPanel, overrides, woxui.KeyEvent{Key: "o", Modifiers: primary, Down: true, Composing: true}) {
		t.Fatal("IME composition must not match built-in hotkeys")
	}
	if matchesBuiltinHotkey(builtinHotkeyActionPanel, nil, woxui.KeyEvent{Key: "o", Modifiers: primary, Down: true}) {
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
