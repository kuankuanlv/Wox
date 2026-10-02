package launcher

import (
	"runtime"
	"strings"

	woxui "wox/ui/runtime"
)

// builtinHotkeyPrefix keys the per-action overrides persisted in
// WoxSetting.BuiltinHotkeyOverrides. Form fields carry keys of the shape
// "BuiltinHotkey.about", and the recording save path forwards the same key
// verbatim to UpdateGeneralSetting.
const builtinHotkeyPrefix = "BuiltinHotkey."

// Built-in launcher-level hotkey IDs. The registry governs the configurable
// app-internal shortcuts (about/settings/actionPanel/quit); preview/webview/notes
// hotkeys stay in their module contexts on purpose (see docs/hotkey-registry-refactor-draft.md §2.2).
const (
	builtinHotkeyAbout       = "about"
	builtinHotkeySettings    = "settings"
	builtinHotkeyActionPanel = "actionPanel"
	builtinHotkeyQuit        = "quit"
)

// builtinHotkeyAction is one registry entry. DefaultKey is the macOS form and
// AltKey the control-based form used on other platforms; both follow the
// "command+shift+k" / "control+shift+k" hotkey string convention.
type builtinHotkeyAction struct {
	ID         string
	LabelKey   string
	TooltipKey string
	DefaultKey string
	AltKey     string
}

// builtinHotkeyDefinitionsList is the single source of truth for configurable
// launcher-level built-in hotkeys: defaults, settings labels, and overview labels.
// All entries are app-own (Wox-window-focused dispatch).
var builtinHotkeyDefinitionsList = []builtinHotkeyAction{
	{ID: builtinHotkeyAbout, LabelKey: "i18n:ui_hotkey_overview_about", TooltipKey: "i18n:ui_builtin_hotkey_about_tips", DefaultKey: "command+shift+k", AltKey: "control+shift+k"},
	{ID: builtinHotkeyActionPanel, LabelKey: "i18n:ui_action_panel_hotkey", TooltipKey: "i18n:ui_action_panel_hotkey_tips", DefaultKey: "command+k", AltKey: "control+k"},
	{ID: builtinHotkeySettings, LabelKey: "i18n:ui_hotkey_overview_settings", TooltipKey: "i18n:ui_builtin_hotkey_settings_tips", DefaultKey: "command+,", AltKey: "control+,"},
	{ID: builtinHotkeyQuit, LabelKey: "i18n:ui_hotkey_overview_quit", TooltipKey: "i18n:ui_builtin_hotkey_quit_tips", DefaultKey: "command+q", AltKey: "control+q"},
}

// builtinHotkeyDefinitions returns a copy of the registry for form/overview
// iteration, keeping the canonical list immutable.
func builtinHotkeyDefinitions() []builtinHotkeyAction {
	return append([]builtinHotkeyAction(nil), builtinHotkeyDefinitionsList...)
}

// builtinHotkeyDefinition looks up one registry entry.
func builtinHotkeyDefinition(id string) (builtinHotkeyAction, bool) {
	for _, def := range builtinHotkeyDefinitionsList {
		if def.ID == id {
			return def, true
		}
	}
	return builtinHotkeyAction{}, false
}

// builtinHotkeyDefaultKey returns the platform default for one registry entry.
func builtinHotkeyDefaultKey(id string) string {
	def, ok := builtinHotkeyDefinition(id)
	if !ok {
		return ""
	}
	if runtime.GOOS == "darwin" {
		return def.DefaultKey
	}
	return def.AltKey
}

// builtinHotkeyForData resolves the effective hotkey for one registry entry
// from a settings snapshot: user override first; the legacy ActionPanelHotkey
// field acts as the actionPanel fallback (zero data migration); platform
// default last.
func builtinHotkeyForData(id string, data settingsData) string {
	if data.BuiltinHotkeyOverrides != nil {
		if value, ok := data.BuiltinHotkeyOverrides[id]; ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if id == builtinHotkeyActionPanel && strings.TrimSpace(data.ActionPanelHotkey) != "" {
		return strings.TrimSpace(data.ActionPanelHotkey)
	}
	return builtinHotkeyDefaultKey(id)
}

// builtinHotkeyForEffective resolves one registry entry against the live
// general settings snapshot.
func (a *App) builtinHotkeyForEffective(id string) string {
	if a == nil || a.generalSettings == nil {
		return builtinHotkeyDefaultKey(id)
	}
	return builtinHotkeyForData(id, a.generalSettings.Data())
}

// matchesBuiltinHotkey reports whether one key event matches a registry entry
// under the given overrides.
func matchesBuiltinHotkey(id string, overrides map[string]string, event woxui.KeyEvent) bool {
	return hotkeyMatches(builtinHotkeyFor(id, overrides), event)
}

// builtinHotkeyFor resolves the effective hotkey for one registry entry:
// user override first, platform default as fallback.
func builtinHotkeyFor(id string, overrides map[string]string) string {
	if overrides != nil {
		if value, ok := overrides[id]; ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return builtinHotkeyDefaultKey(id)
}

// matchesBuiltinHotkeyEffective reports whether one key event matches a
// registry entry resolved against the live settings.
func matchesBuiltinHotkeyEffective(a *App, id string, event woxui.KeyEvent) bool {
	return hotkeyMatches(a.builtinHotkeyForEffective(id), event)
}

// builtinHotkeyOverrides exposes the persisted override map from the general
// settings snapshot. A nil receiver returns nil so callers can pass it straight
// to the registry helpers.
func (a *App) builtinHotkeyOverrides() map[string]string {
	if a != nil && a.generalSettings != nil {
		return a.generalSettings.Data().BuiltinHotkeyOverrides
	}
	return nil
}
