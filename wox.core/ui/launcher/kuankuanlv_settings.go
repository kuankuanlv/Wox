package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	"wox/plugin"
	"wox/ui/dto"
)

// Kuankuanlv extension area labels. This fork feature ships with plain English
// labels because it is not part of the upstream i18n language packs; translate()
// passes non-i18n literals through unchanged.
const (
	kuankuanlvSectionHeader      = "Kuankuanlv Extension"
	kuankuanlvRowSchemaVersion   = "Schema version"
	kuankuanlvRowParameterHint   = "Parameter hint"
	kuankuanlvRowInputDesc       = "Input description"
	kuankuanlvRowOutputDesc      = "Output description"
	kuankuanlvFilterItemsColumn  = "Sub-command"
	kuankuanlvFilterPatternLabel = "Input filter regex pattern"
	kuankuanlvActionIDColumn     = "Action ID"
	kuankuanlvActionHotkeyColumn = "Hotkey"
	kuankuanlvActionLabelColumn  = "Label"
)

// kuankuanlvHasExtension reports whether the plugin declares any kuankuanlv
// fork field. Original (upstream) plugins have every extension field at its
// zero value; for them the settings extension area renders nothing.
func kuankuanlvHasExtension(plugin pluginSettingsPlugin) bool {
	return plugin.KuankuanlvSchemaVersion != 0 ||
		plugin.KuankuanlvInputFilter != nil ||
		plugin.KuankuanlvParameterHint != "" ||
		plugin.KuankuanlvInputDescription != "" ||
		plugin.KuankuanlvOutputDescription != "" ||
		len(plugin.KuankuanlvActions) > 0
}

// kuankuanlvHasWildcardTrigger reports whether the effective trigger keyword
// list contains the global "*" keyword. The input filter area is only shown for
// wildcard plugins.
func kuankuanlvHasWildcardTrigger(plugin pluginSettingsPlugin) bool {
	for _, keyword := range plugin.TriggerKeywords {
		if strings.TrimSpace(keyword) == "*" {
			return true
		}
	}
	return false
}

// kuankuanlvEditableFormDefinitions appends the editable kuankuanlv extension
// fields to the plugin settings form. Every injected field persists through the
// existing plugin setting channel (UpdatePluginSettings -> SaveSetting), reusing
// the wox.db-backed KV store without any new tables or direct database access.
func kuankuanlvEditableFormDefinitions(plugin pluginSettingsPlugin) []formDefinition {
	var definitions []formDefinition

	// Input filter editor: list mode -> editable sub-command table; regex mode ->
	// editable pattern textbox. Only wildcard plugins with a declared filter get
	// an editor.
	if filter := plugin.KuankuanlvInputFilter; filter != nil && kuankuanlvHasWildcardTrigger(plugin) {
		switch {
		case filter.IsList():
			definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
				Key:        dto.KuankuanlvSettingInputFilterItems,
				InlineTable: true,
				MaxHeight:  200,
				Columns: []formTableColumn{{
					Key: "item", Label: kuankuanlvFilterItemsColumn, Type: "text", TextMaxLines: 1,
					Validators: []formValidator{{Type: "not_empty"}},
				}},
				SortColumnKey: "item",
			}})
		case filter.IsRegex():
			definitions = append(definitions, formDefinition{Type: "textbox", Value: formDefinitionValue{
				Key:      dto.KuankuanlvSettingInputFilterPattern,
				Label:    kuankuanlvFilterPatternLabel,
				MaxLines: 1,
			}})
		}
	}

	// Result action combos editor: an editable (id, hotkey, label) table seeded
	// from the author-declared defaults. Only plugins that declared combos get
	// an editor; user overrides are persisted under the same key.
	if len(plugin.KuankuanlvActions) > 0 {
		definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
			Key:        dto.KuankuanlvSettingActions,
			InlineTable: true,
			MaxHeight:  240,
			Columns: []formTableColumn{
				{Key: "id", Label: kuankuanlvActionIDColumn, Type: "text", TextMaxLines: 1, Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "hotkey", Label: kuankuanlvActionHotkeyColumn, Type: "text", TextMaxLines: 1},
				{Key: "label", Label: kuankuanlvActionLabelColumn, Type: "text", TextMaxLines: 1},
			},
			SortColumnKey: "id",
		}})
	}

	return definitions
}

// encodeKuankuanlvFilterItems converts a metadata sub-command list into the
// table-rows JSON the shared table editor consumes.
func encodeKuankuanlvFilterItems(items []string) string {
	rows := make([]map[string]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		rows = append(rows, map[string]string{"item": item})
	}
	encoded, _ := json.Marshal(rows)
	return string(encoded)
}

// decodeKuankuanlvFilterItemRows converts the table-rows JSON back into the
// plain string list that is persisted as the override value.
func decodeKuankuanlvFilterItemRows(rowsJSON string) ([]string, error) {
	rows, err := decodeFormTableRows(rowsJSON)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(rows))
	for _, row := range rows {
		if item := strings.TrimSpace(fmt.Sprint(row["item"])); item != "" {
			items = append(items, item)
		}
	}
	return items, nil
}

// encodeKuankuanlvActions converts author-declared combos into table-rows JSON.
func encodeKuankuanlvActions(actions []plugin.MetadataAction) string {
	rows := make([]map[string]string, 0, len(actions))
	for _, action := range actions {
		rows = append(rows, map[string]string{
			"id":    strings.TrimSpace(action.Id),
			"hotkey": strings.TrimSpace(action.Hotkey),
			"label": strings.TrimSpace(action.Label),
		})
	}
	encoded, _ := json.Marshal(rows)
	return string(encoded)
}

// persistKuankuanlvActionTableRows converts the table-rows JSON into the
// persisted MetadataAction JSON array used as the user override.
func persistKuankuanlvActionTableRows(rowsJSON string) (string, error) {
	rows, err := decodeFormTableRows(rowsJSON)
	if err != nil {
		return "", err
	}
	actions := make([]plugin.MetadataAction, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(fmt.Sprint(row["id"]))
		if id == "" {
			continue
		}
		actions = append(actions, plugin.MetadataAction{
			Id:     id,
			Hotkey: strings.TrimSpace(fmt.Sprint(row["hotkey"])),
			Label:  strings.TrimSpace(fmt.Sprint(row["label"])),
		})
	}
	encoded, err := json.Marshal(actions)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
