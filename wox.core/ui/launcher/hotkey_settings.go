package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"wox/common/icons"
	"wox/setting"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type ignoredHotkeyApp struct {
	Name     string
	Identity string
	Path     string
	Icon     woxImage
}

// pluginHotkeySummary is one aggregated plugin-declared hotkey shown in the
// plugin hotkeys group of the Hotkey settings tab. It is read-only: edits
// happen in the plugin's own settings page (openPluginHotkeySettings).
type pluginHotkeySummary struct {
	PluginID   string
	PluginName string
	Display    string
}

// collectPluginHotkeySummaries aggregates hotkey fields declared by enabled
// plugins, in plugin order. Legacy dictationHotkey definitions are normalized
// to "hotkey" at load, so only the generic type is matched here.
func collectPluginHotkeySummaries(plugins []pluginSettingsPlugin, translate func(string) string) []pluginHotkeySummary {
	summaries := make([]pluginHotkeySummary, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.IsDisable {
			continue
		}
		for _, field := range plugin.SettingDefinitions {
			if field.Type != "hotkey" {
				continue
			}
			label := translate(field.Value.Label)
			value := plugin.Setting.Settings[field.Value.Key]
			display := plugin.Name + " · " + label + "：" + strings.Join(formatHotkeyLabels(value), " + ")
			summaries = append(summaries, pluginHotkeySummary{
				PluginID: plugin.ID, PluginName: plugin.Name, Display: display,
			})
		}
	}
	return summaries
}

// buildHotkeySettingsPage prepares shared form fields for the pure settings page.
func (a *App) buildHotkeySettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.hotkey.Form == nil {
		return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{Width: width, Height: height, Theme: snapshot.palette})
	}
	innerWidth := max(float32(0), width-72)
	callbacks := formFieldCallbacks{
		idPrefix: "hotkey-settings", focus: a.focusHotkeySettingsField, openTable: a.openHotkeySettingsTable, recordKey: a.recordHotkeySettingsField,
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.hotkey.Form.definitions))
	for index, definition := range snapshot.hotkey.Form.definitions {
		rows = append(rows, woxwidget.Keyed{Key: formFieldRowKey("hotkey-settings", index), Child: a.buildFormField(*snapshot.hotkey.Form, callbacks, snapshot.palette, index, definition, innerWidth, 0)})
	}
	return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{
		Width: width, Height: height, Theme: snapshot.palette, Available: true,
		Rows: rows, KeepVisibleKey: formFieldsKeepVisibleKey("hotkey-settings", *snapshot.hotkey.Form),
	})
}

// newHotkeySettingsForm maps global bindings, app-level hotkeys, query
// hotkeys and plugin-declared hotkeys onto the shared form/table engine,
// grouped by exposure level.
func newHotkeySettingsForm(data settingsData, pluginHotkeys []pluginHotkeySummary) formFieldsState {
	definitions := []formDefinition{
		// Group 1: system-wide hotkeys, active while any app is frontmost.
		{Type: "sectionHead", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_global"}},
		{Type: "label", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_global_tips"}},
		{Type: "hotkey", Value: formDefinitionValue{Key: "MainHotkey", Label: "i18n:ui_hotkey", Tooltip: "i18n:ui_hotkey_tips"}},
	}
	if !data.IsLinuxWaylandSession {
		definitions = append(definitions,
			formDefinition{Type: "hotkey", Value: formDefinitionValue{Key: "SelectionHotkey", Label: "i18n:ui_selection_hotkey", Tooltip: "i18n:ui_selection_hotkey_tips"}},
		)
	}
	definitions = append(definitions,
		formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "ResultBindings", Title: "i18n:ui_result_bindings", Tooltip: "i18n:ui_result_bindings_tips", SortColumnKey: "Title", InlineTable: true,
			Columns: []formTableColumn{
				{Key: "Title", Label: "i18n:ui_result_bindings_title", Tooltip: "i18n:ui_result_bindings_title_tooltip", Width: 220, Type: "text", HideInUpdate: true},
				{Key: "Hotkey", Label: "i18n:ui_result_bindings_hotkey", Tooltip: "i18n:ui_result_bindings_hotkey_tooltip", Width: 140, Type: "hotkey"},
				{Key: "Alias", Label: "i18n:ui_result_bindings_alias", Tooltip: "i18n:ui_result_bindings_alias_tooltip", Width: 140, Type: "text"},
			},
		}},
		// 自定义快捷键指向 keyword（全局层级，位于本层最下方）。
		formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "QueryHotkeysGlobal", Title: "i18n:ui_query_hotkeys_global_title", Tooltip: "i18n:ui_query_hotkeys_tips", SortColumnKey: "Query", InlineTable: true, UpdateDialogWidth: 700,
			Columns: queryHotkeyColumns(),
		}},
		// Group 2: hotkeys that only work while the Wox window is focused.
		formDefinition{Type: "sectionHead", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_app"}},
		formDefinition{Type: "label", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_app_tips"}},
	)
	for _, builtin := range builtinHotkeyDefinitions() {
		definitions = append(definitions, formDefinition{Type: "hotkey", Value: formDefinitionValue{
			Key: builtinHotkeyPrefix + builtin.ID, Label: builtin.LabelKey, Tooltip: builtin.TooltipKey,
		}})
	}
	// 自定义快捷键指向 keyword（应用内层级，位于本层最下方）。
	definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "QueryHotkeysApp", Title: "i18n:ui_query_hotkeys_app_title", Tooltip: "i18n:ui_query_hotkeys_tips", SortColumnKey: "Query", InlineTable: true, UpdateDialogWidth: 700,
		Columns: queryHotkeyColumns(),
	}})
	// Group 3: hotkeys declared by enabled plugins, aggregated read-only.
	// The values live in each plugin's settings page; this group only shows
	// them together and offers a jump to the owning plugin.
	definitions = append(definitions,
		formDefinition{Type: "sectionHead", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_plugin"}},
		formDefinition{Type: "label", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_plugin_tips"}},
	)
	for index, summary := range pluginHotkeys {
		definitions = append(definitions, formDefinition{Type: "pluginHotkey", Value: formDefinitionValue{
			Key: fmt.Sprintf("PluginHotkey.%s.%d", summary.PluginID, index), Label: summary.PluginName, Content: summary.Display, Tooltip: "i18n:ui_hotkey_plugin_open_tips",
		}})
	}
	if len(pluginHotkeys) == 0 {
		definitions = append(definitions, formDefinition{Type: "label", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_plugin_empty"}})
	}
	// Group 4: conflicts and exceptions.
	definitions = append(definitions,
		formDefinition{Type: "sectionHead", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_exception"}},
		formDefinition{Type: "label", Value: formDefinitionValue{Content: "i18n:ui_hotkey_group_exception_tips"}},
	)
	if !data.IsLinuxWaylandSession {
		definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "IgnoredHotkeyApps", Title: "i18n:ui_hotkey_ignore_apps", Tooltip: "i18n:ui_hotkey_ignore_apps_tips", MaxHeight: 220, InlineTable: true,
			Columns: []formTableColumn{{Key: "App", Label: "i18n:ui_hotkey_ignore_apps_app", Tooltip: "i18n:ui_hotkey_ignore_apps_tips", Width: 420, Type: "app", Validators: []formValidator{{Type: "not_empty"}}}},
		}})
	}
	values := map[string]string{
		"MainHotkey":         data.MainHotkey,
		"SelectionHotkey":    data.SelectionHotkey,
		"IgnoredHotkeyApps":  settingsIgnoredHotkeyAppRowsJSON(data.IgnoredHotkeyApps),
		"ResultBindings":     settingsRowsJSON(data.ResultBindings),
		"QueryHotkeysGlobal": settingsQueryHotkeysRowsJSON(data.QueryHotkeys, setting.QueryHotkeyExposeLevelGlobal),
		"QueryHotkeysApp":    settingsQueryHotkeysRowsJSON(data.QueryHotkeys, setting.QueryHotkeyExposeLevelApp),
	}
	for _, builtin := range builtinHotkeyDefinitions() {
		values[builtinHotkeyPrefix+builtin.ID] = builtinHotkeyForData(builtin.ID, data)
	}
	return newFormFieldsState(definitions, values, true)
}

// newGeneralQuerySettingsForm maps query aliases and tray launchers onto General.
func newGeneralQuerySettingsForm(data settingsData) formFieldsState {
	definitions := []formDefinition{{Type: "table", Value: formDefinitionValue{
		Key: "QueryAliases", Title: "i18n:ui_query_shortcuts", Tooltip: "i18n:ui_query_shortcuts_tips", SortColumnKey: "Query", InlineTable: true,
		Columns: []formTableColumn{
			{Key: "Shortcut", Label: "i18n:ui_query_shortcuts_shortcut", Tooltip: "i18n:ui_query_shortcuts_shortcut_tooltip", Width: 120, Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
			{Key: "Query", Label: "i18n:ui_query_shortcuts_query", Tooltip: "i18n:ui_query_shortcuts_query_tooltip", Type: "text", QueryTest: true, Validators: []formValidator{{Type: "not_empty"}}},
			{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 60, Type: "checkbox"},
		},
	}}}
	if !data.IsLinuxWaylandSession {
		definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "TrayQueries", Title: "i18n:ui_tray_queries", Tooltip: "i18n:ui_tray_queries_tips", InlineTable: true,
			Columns: []formTableColumn{
				{Key: "Icon", Label: "i18n:ui_tray_queries_icon", Tooltip: "i18n:ui_tray_queries_icon_tooltip", Width: 40, Type: "woxImage"},
				{Key: "Query", Label: "i18n:ui_tray_queries_query", Tooltip: "i18n:ui_tray_queries_query_tooltip", Type: "text", QueryTest: true, Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "HideQueryBox", Label: "i18n:ui_tray_queries_hide_query_box", Tooltip: "i18n:ui_tray_queries_hide_query_box_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "HideToolbar", Label: "i18n:ui_tray_queries_hide_toolbar", Tooltip: "i18n:ui_tray_queries_hide_toolbar_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "Width", Label: "i18n:ui_tray_queries_width", Tooltip: "i18n:ui_tray_queries_width_tooltip", Width: 40, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
				{Key: "MaxResultCount", Label: "i18n:ui_tray_queries_max_result_count", Tooltip: "i18n:ui_tray_queries_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
				{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 50, Type: "checkbox"},
			},
		}})
	}
	values := map[string]string{
		"QueryAliases": settingsRowsJSON(data.QueryAliases),
		"TrayQueries":  settingsJSONArray(data.TrayQueries),
	}
	return newFormFieldsState(definitions, values, true)
}

func settingsIgnoredHotkeyAppRowsJSON(raw json.RawMessage) string {
	var apps []ignoredHotkeyApp
	if len(raw) > 0 && json.Unmarshal(raw, &apps) != nil {
		return "[]"
	}
	rows := make([]map[string]any, 0, len(apps))
	for _, app := range apps {
		rows = append(rows, map[string]any{"App": app})
	}
	return settingsRowsJSON(rows)
}

func settingsIgnoredHotkeyAppsCoreJSON(value string) (string, error) {
	rows, err := decodeFormTableRows(value)
	if err != nil {
		return "", err
	}
	apps := make([]any, 0, len(rows))
	for _, row := range rows {
		app, exists := row["App"]
		if !exists {
			continue
		}
		apps = append(apps, app)
	}
	encoded, err := json.Marshal(apps)
	if err != nil {
		return "", fmt.Errorf("encode ignored hotkey apps: %w", err)
	}
	return string(encoded), nil
}

// loadHotkeyAppCandidates asks core for platform-specific identities and keeps the picker itself platform-neutral.
// Delegates to the hotkey controller which owns the candidate cache and load status.
func (a *App) loadHotkeyAppCandidates() {
	a.hotkeySettings.ReloadAppCandidates(context.Background(), a.services, a.sessionID)
}

func settingsRowsJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// isQueryHotkeysTableKey reports whether a form table key is one of the two
// split query-hotkey tables (global scope vs app-internal scope).
func isQueryHotkeysTableKey(key string) bool {
	return key == "QueryHotkeysGlobal" || key == "QueryHotkeysApp"
}

// queryHotkeyLevelForKey maps a split query-hotkey table key back to its
// exposure level for the merge helper.
func queryHotkeyLevelForKey(key string) string {
	if key == "QueryHotkeysGlobal" {
		return setting.QueryHotkeyExposeLevelGlobal
	}
	return setting.QueryHotkeyExposeLevelApp
}

// settingsQueryHotkeysRowsJSON serializes only the query hotkeys of one
// exposure level so the Global and App tables stay split by scope.
func settingsQueryHotkeysRowsJSON(items []queryHotkeySetting, level string) string {
	filtered := make([]queryHotkeySetting, 0, len(items))
	for _, item := range items {
		// The zero ExposeLevel behaves as global at registration; surface such
		// legacy rows in the global table so every active hotkey is visible.
		if item.ExposeLevel == level || (level == setting.QueryHotkeyExposeLevelGlobal && item.ExposeLevel == "") {
			filtered = append(filtered, item)
		}
	}
	return settingsRowsJSON(filtered)
}

// applyQueryHotkeysLevelRows merges one exposure level's table rows back into
// the shared list, replacing only rows of that level and keeping the other
// level intact. Rows are pinned to the owning level so each table stays the
// single source for its scope.
func applyQueryHotkeysLevelRows(d *settingsData, raw json.RawMessage, level string) {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return
	}
	kept := make([]queryHotkeySetting, 0, len(d.QueryHotkeys))
	for _, item := range d.QueryHotkeys {
		// The zero ExposeLevel behaves as global both at registration and in the
		// global table; match it the same way so editing a legacy row replaces
		// it instead of leaving a duplicate behind.
		itemLevel := item.ExposeLevel
		if itemLevel == "" {
			itemLevel = setting.QueryHotkeyExposeLevelGlobal
		}
		if itemLevel != level {
			kept = append(kept, item)
		}
	}
	added := make([]queryHotkeySetting, 0, len(rows))
	for _, row := range rows {
		item := queryHotkeySetting{
			Name:              formTableRowString(row, "Name"),
			Hotkey:            formTableRowString(row, "Hotkey"),
			Query:             formTableRowString(row, "Query"),
			IsSilentExecution: formTableRowBool(row, "IsSilentExecution"),
			HideQueryBox:      formTableRowBool(row, "HideQueryBox"),
			HideToolbar:       formTableRowBool(row, "HideToolbar"),
			Width:             formTableRowInt(row, "Width"),
			MaxResultCount:    formTableRowInt(row, "MaxResultCount"),
			Position:          formTableRowString(row, "Position"),
			ExposeLevel:       level,
			Disabled:          formTableRowBool(row, "Disabled"),
		}
		added = append(added, item)
	}
	d.QueryHotkeys = append(kept, added...)
}

func formTableRowString(row map[string]any, key string) string {
	if value, ok := row[key].(string); ok {
		return value
	}
	return ""
}

func formTableRowBool(row map[string]any, key string) bool {
	if value, ok := row[key].(bool); ok {
		return value
	}
	return false
}

func formTableRowInt(row map[string]any, key string) int {
	switch value := row[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	}
	return 0
}

func queryHotkeyPositionOptions() []formOption {
	return []formOption{
		{Label: "i18n:ui_query_position_system_default", Value: string(setting.QueryHotkeyPositionSystemDefault), Icon: fromCoreImage(icons.Get("position.system-default"))},
		{Label: "i18n:ui_query_position_top_left", Value: string(setting.QueryHotkeyPositionTopLeft), Icon: fromCoreImage(icons.Get("position.top-left"))},
		{Label: "i18n:ui_query_position_top_center", Value: string(setting.QueryHotkeyPositionTopCenter), Icon: fromCoreImage(icons.Get("position.top-center"))},
		{Label: "i18n:ui_query_position_top_right", Value: string(setting.QueryHotkeyPositionTopRight), Icon: fromCoreImage(icons.Get("position.top-right"))},
		{Label: "i18n:ui_query_position_middle_left", Value: string(setting.QueryHotkeyPositionMiddleLeft), Icon: fromCoreImage(icons.Get("position.middle-left"))},
		{Label: "i18n:ui_query_position_center", Value: string(setting.QueryHotkeyPositionCenter), Icon: fromCoreImage(icons.Get("position.center"))},
		{Label: "i18n:ui_query_position_middle_right", Value: string(setting.QueryHotkeyPositionMiddleRight), Icon: fromCoreImage(icons.Get("position.middle-right"))},
		{Label: "i18n:ui_query_position_bottom_left", Value: string(setting.QueryHotkeyPositionBottomLeft), Icon: fromCoreImage(icons.Get("position.bottom-left"))},
		{Label: "i18n:ui_query_position_bottom_center", Value: string(setting.QueryHotkeyPositionBottomCenter), Icon: fromCoreImage(icons.Get("position.bottom-center"))},
		{Label: "i18n:ui_query_position_bottom_right", Value: string(setting.QueryHotkeyPositionBottomRight), Icon: fromCoreImage(icons.Get("position.bottom-right"))},
	}
}

// queryHotkeyColumns describes the shared columns of the Global and App
// hotkey tables. The owning table carries the exposure level; a per-row
// ExposeLevel picker was removed because it contradicted the pinned-to-table
// save semantics.
func queryHotkeyColumns() []formTableColumn {
	return []formTableColumn{
		{Key: "Name", Label: "i18n:ui_query_hotkeys_name", Tooltip: "i18n:ui_query_hotkeys_name_tooltip", Width: 130, Type: "text"},
		{Key: "Hotkey", Label: "i18n:ui_query_hotkeys_hotkey", Tooltip: "i18n:ui_query_hotkeys_hotkey_tooltip", Width: 120, Type: "hotkey", Validators: []formValidator{{Type: "not_empty"}}},
		{Key: "Query", Label: "i18n:ui_query_hotkeys_query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip", Type: "queryHotkeyQuery", QueryTest: true, Validators: []formValidator{{Type: "not_empty"}}},
		{Key: "Position", Label: "i18n:ui_query_hotkeys_position", Tooltip: "i18n:ui_query_hotkeys_position_tooltip", Width: 120, Type: "select", HideInTable: true, SelectOptions: queryHotkeyPositionOptions()},
		{Key: "HideQueryBox", Label: "i18n:ui_query_hotkeys_hide_query_box", Tooltip: "i18n:ui_query_hotkeys_hide_query_box_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
		{Key: "HideToolbar", Label: "i18n:ui_query_hotkeys_hide_toolbar", Tooltip: "i18n:ui_query_hotkeys_hide_toolbar_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
		{Key: "Width", Label: "i18n:ui_query_hotkeys_width", Tooltip: "i18n:ui_query_hotkeys_width_tooltip", Width: 50, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
		{Key: "MaxResultCount", Label: "i18n:ui_query_hotkeys_max_result_count", Tooltip: "i18n:ui_query_hotkeys_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
		{Key: "IsSilentExecution", Label: "i18n:ui_query_hotkeys_silent", Tooltip: "i18n:ui_query_hotkeys_silent_tooltip", Width: 40, Type: "checkbox", HideInTable: true},
		{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 60, Type: "checkbox"},
	}
}

// optionalIntegerValidators describes blank-or-integer fields, optionally with an inclusive range.
func optionalIntegerValidators(hasRange bool, min, max int, errorKey string) []formValidator {
	return []formValidator{{
		Type: "is_number",
		Value: formValidatorValue{
			IsInteger: true, Optional: true, HasRange: hasRange, Min: min, Max: max, ErrorKey: errorKey,
		},
	}}
}

// onHotkeySettingsKey moves between shared fields without stealing keys from an active recorder.
func (a *App) onHotkeySettingsKey(event woxui.KeyEvent) bool {
	active := a.settingsOpen && a.settingTab == "hotkey" && a.hotkeySettings.Focused() && a.hotkeySettings.Form() != nil && a.settingsTableEditor == nil
	if !active {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.moveHotkeySettingsFocus(-1)
	case woxui.KeyArrowDown:
		a.moveHotkeySettingsFocus(1)
	case woxui.KeyEnter, woxui.KeySpace, woxui.KeyArrowRight:
		a.activateHotkeySettingsField()
	default:
		return false
	}
	return true
}

func (a *App) moveHotkeySettingsFocus(delta int) {
	fields := a.hotkeySettings.Form()
	if fields == nil || len(fields.definitions) == 0 {
		return
	}
	index := fields.focused
	for step := 0; step < len(fields.definitions); step++ {
		index = (index + delta + len(fields.definitions)) % len(fields.definitions)
		if formDefinitionFocusable(fields.definitions[index]) {
			setFormFieldsFocusLocked(fields, index)
			a.settingRow = index
			break
		}
	}
	a.stopHotkeyRecordingForDifferentField(fields, index)
	a.invalidateSettingsWindow()
}

func (a *App) focusHotkeySettingsField(index int) {
	a.stopHotkeyRecordingForDifferentField(a.hotkeySettings.Form(), index)
	if fields := a.hotkeySettings.Form(); fields != nil && index >= 0 && index < len(fields.definitions) && formDefinitionFocusable(fields.definitions[index]) {
		setFormFieldsFocusLocked(fields, index)
		a.settingRow = index
		a.hotkeySettings.SetFocused(true)
	}
	a.invalidateSettingsWindow()
}

func (a *App) activateHotkeySettingsField() {
	fields := a.hotkeySettings.Form()
	if fields == nil || fields.focused < 0 || fields.focused >= len(fields.definitions) {
		return
	}
	index := fields.focused
	typeName := fields.definitions[index].Type
	if typeName == "hotkey" {
		a.recordHotkeySettingsField(index)
	} else if typeName == "pluginHotkey" {
		a.openPluginHotkeySettings(index)
	} else if typeName == "table" {
		a.openHotkeySettingsTable(index)
	}
}

// openPluginHotkeySettings jumps from an aggregated plugin hotkey row to the
// owning plugin's settings page and selects the plugin.
func (a *App) openPluginHotkeySettings(index int) {
	fields := a.hotkeySettings.Form()
	if fields == nil || index < 0 || index >= len(fields.definitions) {
		return
	}
	key := fields.definitions[index].Value.Key
	raw := strings.TrimPrefix(key, "PluginHotkey.")
	if raw == key {
		return
	}
	// Key shape: PluginHotkey.<pluginID>.<fieldIndex>; strip the trailing index.
	pluginID := raw[:strings.LastIndex(raw, ".")]
	a.selectSettingTab("plugins")
	for pluginIndex, plugin := range a.pluginSettings.Plugins() {
		if plugin.ID == pluginID {
			a.pluginSettings.SetSelected(pluginIndex)
			a.setPluginSelectionLocked(pluginIndex)
			break
		}
	}
}

func (a *App) recordHotkeySettingsField(index int) {
	fields := a.hotkeySettings.Form()
	if fields == nil || index < 0 || index >= len(fields.definitions) {
		return
	}
	key := fields.definitions[index].Value.Key
	a.startHotkeyRecording("hotkey-settings", fields, index, key, nil, nil)
}

func (a *App) openHotkeySettingsTable(index int) {
	if form := a.hotkeySettings.Form(); a.settingsOpen && a.settingTab == "hotkey" && form != nil {
		a.settingRow = index
		a.openFormTableLocked(form, index)
	}
	a.finishOpeningFormTable()
}

// onGeneralQuerySettingsKey routes navigation to the focused query table instead of a built-in row.
func (a *App) onGeneralQuerySettingsKey(event woxui.KeyEvent) bool {
	fields := a.generalSettings.Form()
	if !a.settingsOpen || a.settingTab != "general" || !a.generalSettings.FormFocused() || fields == nil || len(fields.definitions) == 0 || a.settingsTableEditor != nil {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.focusGeneralQuerySettingsField((fields.focused - 1 + len(fields.definitions)) % len(fields.definitions))
	case woxui.KeyArrowDown:
		a.focusGeneralQuerySettingsField((fields.focused + 1) % len(fields.definitions))
	case woxui.KeyEnter, woxui.KeySpace, woxui.KeyArrowRight:
		a.openGeneralQuerySettingsTable(fields.focused)
	default:
		return false
	}
	return true
}

// focusGeneralQuerySettingsField keeps one General query table visible for search and tray edit.
func (a *App) focusGeneralQuerySettingsField(index int) {
	if fields := a.generalSettings.Form(); fields != nil && index >= 0 && index < len(fields.definitions) && formDefinitionFocusable(fields.definitions[index]) {
		setFormFieldsFocusLocked(fields, index)
		a.settingRow = index
		a.generalSettings.SetFormFocused(true)
	}
	a.invalidateSettingsWindow()
}

// openGeneralQuerySettingsTable opens a General query table when that page is current.
func (a *App) openGeneralQuerySettingsTable(index int) {
	if form := a.generalSettings.Form(); a.settingsOpen && a.settingTab == "general" && form != nil {
		a.settingRow = index
		a.openFormTableLocked(form, index)
	}
	a.finishOpeningFormTable()
}

// trayQueryRowIndexFromParam extracts a tray query row index from an open-settings param
// like "tray_queries:2", matching the tray icon context menu's edit target.
func trayQueryRowIndexFromParam(param string) (int, bool) {
	param = strings.TrimSpace(param)
	if !strings.HasPrefix(param, "tray_queries:") {
		return 0, false
	}
	index, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(param, "tray_queries:")))
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

// openTrayQueryEditor opens the settings form table and starts editing one tray query row,
// mirroring the inline table's edit button flow.
func (a *App) openTrayQueryEditor(rowIndex int) {
	fields := a.generalSettings.Form()
	if !a.settingsOpen || a.settingTab != "general" || fields == nil || rowIndex < 0 {
		return
	}
	index := -1
	for candidate, definition := range fields.definitions {
		if definition.Value.Key == "TrayQueries" {
			index = candidate
			break
		}
	}
	if index < 0 {
		return
	}
	// Focus the field first so the settings page keeps the TrayQueries table visible
	// while the row editor opens, mirroring the inline table's edit button flow.
	a.focusGeneralQuerySettingsField(index)
	a.openGeneralQuerySettingsTable(index)
	state := a.activeFormTableEditor()
	if state == nil || state.invalid || rowIndex >= len(state.rows) {
		return
	}
	a.selectFormTableRow(rowIndex)
	a.beginEditFormTableRowDirect()
}

func (a *App) applyHotkeySettingsRawLocked(key, value string) {
	raw := json.RawMessage(append([]byte(nil), value...))
	switch key {
	case "QueryHotkeysGlobal":
		a.generalSettings.Update(func(d *settingsData) { applyQueryHotkeysLevelRows(d, raw, setting.QueryHotkeyExposeLevelGlobal) })
	case "QueryHotkeysApp":
		a.generalSettings.Update(func(d *settingsData) { applyQueryHotkeysLevelRows(d, raw, setting.QueryHotkeyExposeLevelApp) })
	case "ResultBindings":
		a.generalSettings.Update(func(d *settingsData) { _ = json.Unmarshal(raw, &d.ResultBindings) })
	case "IgnoredHotkeyApps":
		a.generalSettings.Update(func(d *settingsData) { d.IgnoredHotkeyApps = raw })
	case "QueryAliases":
		a.generalSettings.Update(func(d *settingsData) { _ = json.Unmarshal(raw, &d.QueryAliases) })
	case "TrayQueries":
		a.generalSettings.Update(func(d *settingsData) { d.TrayQueries = raw })
	}
}
