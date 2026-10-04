package launcher

import (
	"context"
	"encoding/json"
	"strings"

	"wox/plugin"
	"wox/ui/dto"
)

// currentKuankuanlvActionCombos returns the effective result-action combos for
// the plugin that owns the current query, or nil when the query is a global
// query (results may come from many plugins) or the plugin declared no combos.
// User overrides saved through the settings form (same plugin setting KV channel
// as InputFilter) replace the author-declared metadata defaults.
//
// This is UI-layer projection only: combos are matched against result actions
// by ID and hotkeys are attached for the existing keyboard dispatch chain to
// consume. No system hotkeys are registered and nothing is written into the
// hotkey registry.
func (a *App) currentKuankuanlvActionCombos() []plugin.MetadataAction {
	pluginID := strings.TrimSpace(a.queryContext.PluginID)
	if pluginID == "" {
		return nil
	}
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance == nil || strings.TrimSpace(instance.Metadata.Id) != pluginID {
			continue
		}
		combos := append([]plugin.MetadataAction(nil), instance.Metadata.KuankuanlvActions...)
		if instance.API != nil {
			if override := strings.TrimSpace(instance.API.GetSetting(context.Background(), dto.KuankuanlvSettingActions)); override != "" {
				var parsed []plugin.MetadataAction
				if json.Unmarshal([]byte(override), &parsed) == nil {
					combos = parsed
				}
			}
		}
		return combos
	}
	return nil
}

// projectKuankuanlvActions attaches combo-declared hotkeys to result actions by
// matching action IDs. Result actions that already carry their own hotkey
// (written by plugin SDK code) are never overwritten; actions with no matching
// combo are untouched. With no combos or no actions it returns the input
// unchanged, so upstream behavior is preserved.
func projectKuankuanlvActions(actions []resultAction, combos []plugin.MetadataAction) []resultAction {
	if len(combos) == 0 || len(actions) == 0 {
		return actions
	}
	hotkeyByID := make(map[string]string, len(combos))
	for _, combo := range combos {
		id := strings.TrimSpace(combo.Id)
		if id == "" {
			continue
		}
		hotkeyByID[id] = strings.TrimSpace(combo.Hotkey)
	}
	projected := make([]resultAction, len(actions))
	copy(projected, actions)
	for i := range projected {
		if projected[i].Hotkey != "" {
			continue
		}
		if hotkey, ok := hotkeyByID[strings.TrimSpace(projected[i].ID)]; ok && hotkey != "" {
			projected[i].Hotkey = hotkey
		}
	}
	return projected
}

// resultsWithProjectedActions returns a shallow copy of a.results with combo
// hotkeys projected onto each result's actions. It never mutates a.results, so
// panel rendering and hotkey dispatch stay consistent with the live state.
func (a *App) resultsWithProjectedActions() []queryResult {
	combos := a.currentKuankuanlvActionCombos()
	if len(combos) == 0 {
		return a.results
	}
	projected := make([]queryResult, len(a.results))
	copy(projected, a.results)
	for i := range projected {
		projected[i].Actions = projectKuankuanlvActions(projected[i].Actions, combos)
	}
	return projected
}
