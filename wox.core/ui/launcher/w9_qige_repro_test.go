package launcher

import (
	"encoding/json"
	"testing"
)

// Reproduce qigeTools-style commands table: two fixed-URL rows, SortColumnKey=ActionType.
// Editing one row's Template must never touch the other row, and the saved JSON must
// keep each row's Keyword/Template independent.
func TestCommandsRowEditDoesNotCrossContaminate(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{
		Key:        "commands",
		SortColumnKey: "ActionType",
		SortOrder:   "asc",
		Columns: []formTableColumn{
			{Key: "Keyword", Type: "text"},
			{Key: "ActionType", Type: "select", SelectOptions: []formOption{
				{Label: "url", Value: "url"},
				{Label: "folder", Value: "folder"},
			}},
			{Key: "Template", Type: "text"},
			{Key: "Subtitle", Type: "text"},
		},
	}}
	initialRows := []map[string]any{
		{"Keyword": "cd-dev", "ActionType": "url", "Template": "https://dev.example.com", "Subtitle": "dev"},
		{"Keyword": "cd-test", "ActionType": "url", "Template": "https://test.example.com", "Subtitle": "test"},
	}
	encoded, _ := json.Marshal(initialRows)
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"commands": string(encoded)}, true)

	rows, err := decodeFormTableRows(string(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	app := &App{}
	state := &formTableEditorState{target: &target, definition: definition, rows: rows, selected: 0, rowIndex: -1, deletePending: -1}
	app.launcherTableEditor = state

	// User selects & edits the second row (cd-test), changes only its Template.
	app.selectFormTableRow(1)
	if state.selected != 1 {
		t.Fatalf("selected = %d, want 1", state.selected)
	}
	app.beginFormTableRowEdit(1, false, false)
	if state.rowForm == nil {
		t.Fatal("rowForm not initialized")
	}
	if state.rowBase["Keyword"] != "cd-test" {
		t.Fatalf("rowBase Keyword = %v, want cd-test (editing wrong row)", state.rowBase["Keyword"])
	}
	state.rowForm.values["Template"] = "https://test.example.com/updated"

	app.replaceFormTableEditorRows(state, []map[string]any{formTableRowFromFields(state.definition, state.rowForm, state.rowBase)})
	if err := app.commitFormTableRowsLocked(state); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if len(state.rows) != 2 {
		t.Fatalf("rows count = %d, want 2", len(state.rows))
	}
	r0, r1 := state.rows[0], state.rows[1]
	if r0["Keyword"] != "cd-dev" || r0["Template"] != "https://dev.example.com" {
		t.Fatalf("row0 contaminated: %#v", r0)
	}
	if r1["Keyword"] != "cd-test" || r1["Template"] != "https://test.example.com/updated" {
		t.Fatalf("row1 wrong: %#v", r1)
	}

	// Saved JSON round-trips both rows independently.
	saved, err := decodeFormTableRows(state.target.values["commands"])
	if err != nil {
		t.Fatalf("saved decode: %v", err)
	}
	if saved[0]["Keyword"] != "cd-dev" || saved[1]["Keyword"] != "cd-test" {
		t.Fatalf("saved keywords = %v,%v", saved[0]["Keyword"], saved[1]["Keyword"])
	}
	if saved[0]["Template"] != "https://dev.example.com" || saved[1]["Template"] != "https://test.example.com/updated" {
		t.Fatalf("saved templates cross-contaminated: %#v", saved)
	}
}

// When SortColumnKey reorders rows, tapping a displayed position must resolve to
// the row actually shown there (via rowViewOrder), not the raw list index.
func TestFormTableRowSelectUsesSortedViewOrder(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "commands", SortColumnKey: "ActionType", SortOrder: "asc",
		Columns: []formTableColumn{
			{Key: "Keyword", Type: "text"},
			{Key: "ActionType", Type: "select", SelectOptions: []formOption{
				{Label: "url", Value: "url"}, {Label: "folder", Value: "folder"},
			}},
			{Key: "Template", Type: "text"},
		},
	}}
	// Original order: row0=url, row1=folder. Sort asc puts folder(row1) first (folder < url), so display order differs from raw order.
	rows := []map[string]any{
		{"Keyword": "cd-test", "ActionType": "url", "Template": "https://test"},
		{"Keyword": "cd-dev", "ActionType": "folder", "Template": "dir-dev"},
	}
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"commands": "[]"}, true)
	app := &App{}
	state := &formTableEditorState{target: &target, definition: definition, rows: rows, selected: 0, rowIndex: -1, deletePending: -1}
	app.launcherTableEditor = state

	viewOrder := formTableSortedRowIndices(definition, rows)
	if viewOrder[0] != 1 || viewOrder[1] != 0 {
		t.Fatalf("viewOrder = %v, want [1 0] (folder row shown first)", viewOrder)
	}
	state.rowViewOrder = viewOrder

	// User taps the FIRST displayed row (which is the folder row, original index 1).
	app.selectFormTableRow(0)
	if state.selected != 1 {
		t.Fatalf("after tapping displayed row 0, selected = %d, want 1 (folder row); sorted-row selection resolved to the wrong underlying row", state.selected)
	}
}
