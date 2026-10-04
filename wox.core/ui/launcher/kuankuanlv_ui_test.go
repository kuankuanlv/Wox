package launcher

import (
	"encoding/json"
	"testing"

	"wox/plugin"
	"wox/ui/dto"
)

func TestKeywordCompletionCandidate(t *testing.T) {
	keywords := []registeredTriggerKeyword{
		{keyword: "search", pluginID: "com.search"},
		{keyword: "snippet", pluginID: "com.snippet"},
		{keyword: "*", pluginID: "com.global"},
	}
	cases := []struct {
		name         string
		text         string
		queryPlugin  string
		keywords     []registeredTriggerKeyword
		want         string
	}{
		{"prefix picks shortest", "se", "", keywords, "search"},
		{"exact keyword does not complete", "search", "", keywords, ""},
		{"wildcard never completed", "*", "", []registeredTriggerKeyword{{keyword: "*", pluginID: "x"}}, ""},
		{"prefer query plugin", "s", "com.snippet", keywords, "snippet"},
		{"no match", "xyz", "", keywords, ""},
		{"empty text", "", "", keywords, ""},
		{"equal length tie breaks alphabetically", "se", "", []registeredTriggerKeyword{{keyword: "seb", pluginID: "b"}, {keyword: "sea", pluginID: "a"}}, "sea"},
		{"length wins over alphabet", "se", "", []registeredTriggerKeyword{{keyword: "searches", pluginID: "a"}, {keyword: "seed", pluginID: "b"}}, "seed"},
		{"query plugin without prefix match falls back", "se", "com.other", keywords, "search"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := keywordCompletionCandidate(tc.text, tc.keywords, tc.queryPlugin); got != tc.want {
				t.Fatalf("keywordCompletionCandidate(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestProjectKuankuanlvActions(t *testing.T) {
	actions := []resultAction{{ID: "open"}, {ID: "copy"}, {ID: "share", Hotkey: "cmd+enter"}}
	combos := []plugin.MetadataAction{
		{Id: "open", Hotkey: "enter", Label: "Open"},
		{Id: "copy", Hotkey: "shift+enter", Label: "Copy"},
	}
	projected := projectKuankuanlvActions(actions, combos)
	if projected[0].Hotkey != "enter" {
		t.Fatalf("open hotkey = %q, want enter", projected[0].Hotkey)
	}
	if projected[1].Hotkey != "shift+enter" {
		t.Fatalf("copy hotkey = %q, want shift+enter", projected[1].Hotkey)
	}
	// SDK-written hotkey must never be overwritten.
	if projected[2].Hotkey != "cmd+enter" {
		t.Fatalf("share hotkey = %q, want cmd+enter preserved", projected[2].Hotkey)
	}

	// No combos: actions returned unchanged (upstream default Enter behavior).
	if got := projectKuankuanlvActions(actions, nil); len(got) != 3 || got[0].Hotkey != "" || got[2].Hotkey != "cmd+enter" {
		t.Fatalf("nil combos must leave actions untouched: %#v", got)
	}
	// Empty actions: no panic, no allocations worth testing.
	if got := projectKuankuanlvActions(nil, combos); got != nil {
		t.Fatalf("nil actions = %#v, want nil", got)
	}
}

func TestKuankuanlvExtensionSectionEligibility(t *testing.T) {
	// Original plugin: every extension field at zero -> no extension area.
	original := pluginSettingsPlugin{ID: "com.old", TriggerKeywords: []string{"old"}}
	if kuankuanlvHasExtension(original) {
		t.Fatal("original plugin must report no extension")
	}
	if kuankuanlvHasWildcardTrigger(original) {
		t.Fatal("original keyword plugin must not be wildcard")
	}
	if defs := kuankuanlvEditableFormDefinitions(original); len(defs) != 0 {
		t.Fatalf("original plugin must inject no editable definitions, got %d", len(defs))
	}

	// Wildcard plugin with a list-mode filter gets an items table editor.
	wildcard := pluginSettingsPlugin{
		ID:              "com.wild",
		TriggerKeywords: []string{"*"},
		KuankuanlvInputFilter: &plugin.MetadataInputFilter{Mode: "list", Items: []string{"calc", "weather"}},
	}
	if !kuankuanlvHasExtension(wildcard) || !kuankuanlvHasWildcardTrigger(wildcard) {
		t.Fatal("wildcard list plugin must report extension + wildcard")
	}
	defs := kuankuanlvEditableFormDefinitions(wildcard)
	if len(defs) != 1 || defs[0].Type != "table" || defs[0].Value.Key != dto.KuankuanlvSettingInputFilterItems {
		t.Fatalf("list-mode wildcard plugin must inject items table, got %#v", defs)
	}

	// Wildcard plugin with a regex filter gets a textbox editor.
	regex := pluginSettingsPlugin{
		ID:              "com.regex",
		TriggerKeywords: []string{"*"},
		KuankuanlvInputFilter: &plugin.MetadataInputFilter{Mode: "regex", Pattern: `^g\s+\w+`},
	}
	if defs := kuankuanlvEditableFormDefinitions(regex); len(defs) != 1 || defs[0].Type != "textbox" || defs[0].Value.Key != dto.KuankuanlvSettingInputFilterPattern {
		t.Fatalf("regex-mode wildcard plugin must inject pattern textbox, got %#v", defs)
	}

	// Declared filter on a non-wildcard plugin must NOT get an editor.
	nonWild := pluginSettingsPlugin{
		ID:                    "com.nonwild",
		TriggerKeywords:       []string{"kw"},
		KuankuanlvInputFilter: &plugin.MetadataInputFilter{Mode: "list", Items: []string{"a"}},
	}
	if defs := kuankuanlvEditableFormDefinitions(nonWild); len(defs) != 0 {
		t.Fatalf("non-wildcard plugin must not get an input filter editor, got %#v", defs)
	}
}

func TestKuankuanlvActionEditorOnlyWhenDeclared(t *testing.T) {
	withActions := pluginSettingsPlugin{
		ID:              "com.actions",
		TriggerKeywords: []string{"*"},
		KuankuanlvActions: []plugin.MetadataAction{
			{Id: "copy", Hotkey: "shift+enter", Label: "Copy"},
		},
	}
	defs := kuankuanlvEditableFormDefinitions(withActions)
	if len(defs) != 1 || defs[0].Value.Key != dto.KuankuanlvSettingActions {
		t.Fatalf("plugin with declared actions must inject actions table, got %#v", defs)
	}
}

func TestKuankuanlvFilterItemsRoundTrip(t *testing.T) {
	encoded := encodeKuankuanlvFilterItems([]string{"calc", "", "  weather  "})
	var rows []map[string]string
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		t.Fatalf("encoded rows must be valid JSON: %v", err)
	}
	items, err := decodeKuankuanlvFilterItemRows(encoded)
	if err != nil {
		t.Fatalf("decode rows: %v", err)
	}
	if len(items) != 2 || items[0] != "calc" || items[1] != "weather" {
		t.Fatalf("round trip = %#v, want [calc weather]", items)
	}
}

func TestKuankuanlvActionsRoundTrip(t *testing.T) {
	actions := []plugin.MetadataAction{
		{Id: "open", Hotkey: "enter", Label: "Open"},
		{Id: "copy", Hotkey: "shift+enter", Label: "Copy"},
	}
	encoded := encodeKuankuanlvActions(actions)
	persisted, err := persistKuankuanlvActionTableRows(encoded)
	if err != nil {
		t.Fatalf("persist rows: %v", err)
	}
	var parsed []plugin.MetadataAction
	if err := json.Unmarshal([]byte(persisted), &parsed); err != nil {
		t.Fatalf("persisted override must be MetadataAction JSON: %v", err)
	}
	if len(parsed) != 2 || parsed[0].Id != "open" || parsed[0].Hotkey != "enter" || parsed[1].Hotkey != "shift+enter" {
		t.Fatalf("actions round trip = %#v", parsed)
	}
}
