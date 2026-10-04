package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"wox/util"
)

// kuankuanlvAdmissionLogger ensures the package logger is non-nil in bare unit
// tests that never boot the global plugin manager (the fail-open warning path
// logs through it).
func kuankuanlvAdmissionLogger() {
	if logger == nil {
		logger = util.GetLogger()
	}
}

func newKuankuanlvInstance(id string, keywords []string, filter *MetadataInputFilter, hint string) *Instance {
	return &Instance{
		Metadata: Metadata{
			Id:                      id,
			TriggerKeywords:         keywords,
			KuankuanlvInputFilter:   filter,
			KuankuanlvParameterHint: hint,
		},
	}
}

func kuankuanlvInputQuery(raw string, triggerKeyword string) Query {
	return Query{
		Type:           QueryTypeInput,
		RawQuery:       raw,
		TriggerKeyword: triggerKeyword,
	}
}

// TestKuankuanlvInputFilterListAdmission covers list-mode bidirectional,
// case-insensitive prefix admission for wildcard ("*") plugins.
func TestKuankuanlvInputFilterListAdmission(t *testing.T) {
	m := &Manager{}
	ctx := context.Background()
	inst := newKuankuanlvInstance("test.list-filter", []string{"*"},
		&MetadataInputFilter{Mode: "list", Items: []string{"open", "close"}}, "")

	// exact item -> admitted
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("open", "")))
	// item is a prefix of what the user typed -> admitted
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("op", "")))
	// typed input is a prefix of an item (typing into the middle) -> admitted
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("clos", "")))
	// case-insensitive
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("OPEN", "")))
	// unrelated input -> not admitted
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("xyz", "")))
}

// TestKuankuanlvInputFilterRegexAdmission covers regex-mode admission.
func TestKuankuanlvInputFilterRegexAdmission(t *testing.T) {
	m := &Manager{}
	ctx := context.Background()
	inst := newKuankuanlvInstance("test.regex-filter", []string{"*"},
		&MetadataInputFilter{Mode: "regex", Pattern: "^cli (add|del)$"}, "")

	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("cli add", "")))
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("cli remove", "")))
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("add", "")))
}

// TestKuankuanlvNilInputFilterPassesThrough: an invalid regex was already dropped
// to nil by the metadata validator, so the wildcard plugin keeps upstream
// behavior (admits every input).
func TestKuankuanlvNilInputFilterPassesThrough(t *testing.T) {
	m := &Manager{}
	ctx := context.Background()
	inst := newKuankuanlvInstance("test.nil-filter", []string{"*"}, nil, "")
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("anything at all", "")))
}

// TestKuankuanlvWildcardExcludedOnSpacedKeyword: a space-separated parameter
// query stays keyword-exclusive; the wildcard channel must not participate,
// while a bare keyword still mixes wildcard results in.
func TestKuankuanlvWildcardExcludedOnSpacedKeyword(t *testing.T) {
	m := &Manager{}
	ctx := context.Background()
	inst := newKuankuanlvInstance("test.wild", []string{"*"}, nil, "")

	// keyword-targeted parameter query: "*" excluded
	spaced := kuankuanlvInputQuery("kw hello", "kw")
	spaced.Search = "hello"
	assert.False(t, m.canOperateQuery(ctx, inst, spaced))

	// bare keyword: "*" still participates (upstream behavior preserved)
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("kw", "kw")))
}

// TestKuankuanlvParameterHintResults covers the core-owned hint row for ordinary
// keyword plugins: declared hint produces a row; trailing space / no hint /
// wildcard guard suppress it.
func TestKuankuanlvParameterHintResults(t *testing.T) {
	m := &Manager{}
	ctx := context.Background()

	ordinary := newKuankuanlvInstance("test.hint", []string{"calc"}, nil, "<expr>")
	hits := m.kuankuanlvParameterHintResults(ctx, ordinary, kuankuanlvInputQuery("calc", "calc"))
	require.Len(t, hits, 1)
	assert.Equal(t, "<expr>", hits[0].Title)
	assert.Equal(t, kuankuanlvParameterHintScoreKey, hits[0].ScoreKey)

	// trailing space: parameter-input state, plugin prompts for itself
	spaced := kuankuanlvInputQuery("calc ", "calc")
	assert.Empty(t, m.kuankuanlvParameterHintResults(ctx, ordinary, spaced))

	// hint declared empty: plain dispatch, no hint row
	noHint := newKuankuanlvInstance("test.no-hint", []string{"calc"}, nil, "")
	assert.Empty(t, m.kuankuanlvParameterHintResults(ctx, noHint, kuankuanlvInputQuery("calc", "calc")))

	// wildcard guard (hand-built instance bypassing the validator): never a hint row
	wild := newKuankuanlvInstance("test.wild-hint", []string{"*"}, nil, "<expr>")
	assert.Empty(t, m.kuankuanlvParameterHintResults(ctx, wild, kuankuanlvInputQuery("calc", "calc")))
}

// TestKuankuanlvRegisterInputFilter covers runtime registration: shadowing the
// metadata filter, regex cache rebuild on re-register, unregister falling back
// to metadata, and an uncompilable runtime regex failing open.
func TestKuankuanlvRegisterInputFilter(t *testing.T) {
	m := &Manager{}
	ctx := context.Background()
	inst := newKuankuanlvInstance("test.runtime", []string{"*"},
		&MetadataInputFilter{Mode: "list", Items: []string{"meta"}}, "")

	// metadata filter active initially
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("meta", "")))
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("rt", "")))

	// runtime registration shadows the metadata filter
	inst.RegisterInputFilter(&MetadataInputFilter{Mode: "list", Items: []string{"rt"}})
	require.NotNil(t, inst.GetEffectiveInputFilter())
	assert.Equal(t, []string{"rt"}, inst.GetEffectiveInputFilter().Items)
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("meta", "")), "runtime filter shadows metadata")
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("rt", "")))

	// regex cache must be rebuilt when the pattern is re-registered
	inst.RegisterInputFilter(&MetadataInputFilter{Mode: "regex", Pattern: "^zz$"})
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("zz", "")))
	inst.RegisterInputFilter(&MetadataInputFilter{Mode: "regex", Pattern: "^zzz$"})
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("zz", "")), "stale compiled regex must not survive re-register")
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("zzz", "")))

	// unregister clears the runtime override and falls back to metadata
	inst.UnregisterInputFilter()
	require.NotNil(t, inst.GetEffectiveInputFilter())
	assert.Equal(t, []string{"meta"}, inst.GetEffectiveInputFilter().Items)
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("meta", "")))
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("rt", "")))

	// runtime-registered uncompilable regex fails open instead of silencing input
	kuankuanlvAdmissionLogger()
	inst.RegisterInputFilter(&MetadataInputFilter{Mode: "regex", Pattern: "([unclosed"})
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("anything", "")), "bad runtime regex must fail open")
}
