package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKuankuanlvMetadataParsesAllExtensions verifies that a plugin.json carrying
// every kuankuanlv extension field unmarshals into the correct Go values (no
// validation is run here — raw parsing is under test).
func TestKuankuanlvMetadataParsesAllExtensions(t *testing.T) {
	pluginJSON := `{
		"id": "test.all-extensions",
		"triggerKeywords": ["kw"],
		"runtime": "node",
		"supportedOS": ["Macos"],
		"kuankuanlv_schema_version": 2,
		"kuankuanlv_input_filter": {"mode": "regex", "items": [], "pattern": "^abc$"},
		"kuankuanlv_parameter_hint": "<file>",
		"kuankuanlv_input_description": "the file path the user typed",
		"kuankuanlv_output_description": "matching files",
		"kuankuanlv_actions": [
			{"id": "open", "hotkey": "enter", "label": "Open"},
			{"id": "reveal", "hotkey": "ctrl+enter", "label": "Reveal"}
		]
	}`

	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(pluginJSON), &m))

	assert.Equal(t, 2, m.KuankuanlvSchemaVersion)
	require.NotNil(t, m.KuankuanlvInputFilter)
	assert.Equal(t, "regex", m.KuankuanlvInputFilter.Mode)
	assert.Equal(t, "^abc$", m.KuankuanlvInputFilter.Pattern)
	assert.Equal(t, "<file>", m.KuankuanlvParameterHint)
	assert.Equal(t, "the file path the user typed", m.KuankuanlvInputDescription)
	assert.Equal(t, "matching files", m.KuankuanlvOutputDescription)
	require.Len(t, m.KuankuanlvActions, 2)
	assert.Equal(t, MetadataAction{Id: "open", Hotkey: "enter", Label: "Open"}, m.KuankuanlvActions[0])
	assert.Equal(t, MetadataAction{Id: "reveal", Hotkey: "ctrl+enter", Label: "Reveal"}, m.KuankuanlvActions[1])
}

// TestKuankuanlvSchemaVersionDefaultsToOne verifies an unset (0) schema version
// is treated as 1 after validation, and an explicitly set value is preserved.
func TestKuankuanlvSchemaVersionDefaultsToOne(t *testing.T) {
	ctx := context.Background()

	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "test.schema-default",
		"triggerKeywords": ["*"],
		"runtime": "node",
		"supportedOS": ["Macos"]
	}`), &m))
	assert.Equal(t, 0, m.KuankuanlvSchemaVersion, "zero value before validation")

	require.NoError(t, m.ValidateKuankuanlvExtensions(ctx))
	assert.Equal(t, 1, m.KuankuanlvSchemaVersion, "0 must be normalized to 1")

	// Explicitly declared version is kept.
	m.KuankuanlvSchemaVersion = 3
	require.NoError(t, m.ValidateKuankuanlvExtensions(ctx))
	assert.Equal(t, 3, m.KuankuanlvSchemaVersion)
}

// TestKuankuanlvExtensionsAbsentInUpstreamPluginJSON verifies that an upstream
// plugin.json (no kuankuanlv fields at all) parses to zero/nil extension values
// and that validation never errors.
func TestKuankuanlvExtensionsAbsentInUpstreamPluginJSON(t *testing.T) {
	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "test.upstream-style",
		"triggerKeywords": ["kw"],
		"runtime": "node",
		"supportedOS": ["Macos"]
	}`), &m))

	// zero/nil before validation
	assert.Equal(t, 0, m.KuankuanlvSchemaVersion)
	assert.Nil(t, m.KuankuanlvInputFilter)
	assert.Equal(t, "", m.KuankuanlvParameterHint)
	assert.Equal(t, "", m.KuankuanlvInputDescription)
	assert.Equal(t, "", m.KuankuanlvOutputDescription)
	assert.Nil(t, m.KuankuanlvActions)

	// validation must not error and must only default the schema version
	require.NoError(t, m.ValidateKuankuanlvExtensions(context.Background()))
	assert.Equal(t, 1, m.KuankuanlvSchemaVersion)
	assert.Nil(t, m.KuankuanlvInputFilter)
	assert.Equal(t, "", m.KuankuanlvParameterHint)
	assert.Nil(t, m.KuankuanlvActions)
}

// TestKuankuanlvValidationIgnoresInvalidCombinations covers the five loose-binding
// rules. Each invalid combination is silently ignored (field reset) and the method
// never returns an error.
func TestKuankuanlvValidationIgnoresInvalidCombinations(t *testing.T) {
	ctx := context.Background()

	t.Run("wildcard plugin with parameter hint", func(t *testing.T) {
		m := Metadata{
			Id:                      "test.wild-hint",
			TriggerKeywords:         []string{"*"},
			KuankuanlvParameterHint: "should-be-dropped",
		}
		require.NoError(t, m.ValidateKuankuanlvExtensions(ctx))
		assert.Equal(t, "", m.KuankuanlvParameterHint, "parameter hint must be dropped for wildcard plugin")
	})

	t.Run("ordinary keyword with input filter", func(t *testing.T) {
		m := Metadata{
			Id:                    "test.kw-filter",
			TriggerKeywords:       []string{"kw"},
			KuankuanlvInputFilter: &MetadataInputFilter{Mode: "list", Items: []string{"a", "b"}},
		}
		require.NoError(t, m.ValidateKuankuanlvExtensions(ctx))
		assert.Nil(t, m.KuankuanlvInputFilter, "input filter must be dropped for non-wildcard plugin")
	})

	t.Run("input filter with invalid mode", func(t *testing.T) {
		m := Metadata{
			Id:                    "test.bad-mode",
			TriggerKeywords:       []string{"*"},
			KuankuanlvInputFilter: &MetadataInputFilter{Mode: "glob", Items: []string{"a"}},
		}
		require.NoError(t, m.ValidateKuankuanlvExtensions(ctx))
		assert.Nil(t, m.KuankuanlvInputFilter, "input filter must be dropped for unknown mode")
	})

	t.Run("regex mode with invalid pattern", func(t *testing.T) {
		m := Metadata{
			Id:                    "test.bad-regex",
			TriggerKeywords:       []string{"*"},
			KuankuanlvInputFilter: &MetadataInputFilter{Mode: "regex", Pattern: "([unclosed"},
		}
		require.NoError(t, m.ValidateKuankuanlvExtensions(ctx))
		assert.Nil(t, m.KuankuanlvInputFilter, "input filter must be dropped for uncompilable regex")
	})

	t.Run("list mode with empty items", func(t *testing.T) {
		m := Metadata{
			Id:                    "test.empty-items",
			TriggerKeywords:       []string{"*"},
			KuankuanlvInputFilter: &MetadataInputFilter{Mode: "list", Items: []string{}},
		}
		require.NoError(t, m.ValidateKuankuanlvExtensions(ctx))
		assert.Nil(t, m.KuankuanlvInputFilter, "input filter must be dropped when list items are empty")
	})
}

// TestKuankuanlvValidFilterIsKept ensures a well-formed wildcard input filter
// survives validation (only the invalid combos are dropped).
func TestKuankuanlvValidFilterIsKept(t *testing.T) {
	m := Metadata{
		Id:                    "test.valid-filter",
		TriggerKeywords:       []string{"*"},
		KuankuanlvInputFilter: &MetadataInputFilter{Mode: "list", Items: []string{"open", "close"}},
	}
	require.NoError(t, m.ValidateKuankuanlvExtensions(context.Background()))
	require.NotNil(t, m.KuankuanlvInputFilter)
	assert.Equal(t, "list", m.KuankuanlvInputFilter.Mode)
	assert.Equal(t, []string{"open", "close"}, m.KuankuanlvInputFilter.Items)
}

// TestKuankuanlvInputFilterConvenienceMethods verifies IsList/IsRegex mode
// detection, including nil-safety and the "invalid mode = ineffective" contract.
func TestKuankuanlvInputFilterConvenienceMethods(t *testing.T) {
	list := &MetadataInputFilter{Mode: "list"}
	assert.True(t, list.IsList())
	assert.False(t, list.IsRegex())

	regex := &MetadataInputFilter{Mode: "regex"}
	assert.True(t, regex.IsRegex())
	assert.False(t, regex.IsList())

	bad := &MetadataInputFilter{Mode: "glob"}
	assert.False(t, bad.IsList())
	assert.False(t, bad.IsRegex())

	var nilFilter *MetadataInputFilter
	assert.False(t, nilFilter.IsList())
	assert.False(t, nilFilter.IsRegex())
}
