package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKuankuanlvBareKeywordCandidateDecision verifies the Phase 1 candidate
// channel decision of the four-phase main search: a bare keyword (no space yet)
// becomes candidate-only; space-separated parameters, commands, scope and
// selection queries stay immediate; wildcard "*" plugins keep computing.
func TestKuankuanlvBareKeywordCandidateDecision(t *testing.T) {
	ordinary := newKuankuanlvInstance("test.cand", []string{"uuid"}, nil, "")
	wild := newKuankuanlvInstance("test.wild", []string{"*"}, nil, "")

	// bare keyword -> candidate channel
	assert.True(t, isBareKeywordQuery(kuankuanlvInputQuery("uuid", "uuid"), ordinary))
	// partial prefix typing -> still candidate channel
	assert.True(t, isBareKeywordQuery(kuankuanlvInputQuery("uu", "uu"), ordinary))

	// space-separated parameter -> immediate computation
	spaced := kuankuanlvInputQuery("uuid ", "uuid")
	spaced.Search = ""
	assert.False(t, isBareKeywordQuery(spaced, ordinary))
	spacedParam := kuankuanlvInputQuery("uuid abc", "uuid")
	spacedParam.Search = "abc"
	assert.False(t, isBareKeywordQuery(spacedParam, ordinary))

	// wildcard plugins never use the candidate channel
	assert.False(t, isBareKeywordQuery(kuankuanlvInputQuery("wech", ""), wild))

	// command queries stay immediate
	cmd := kuankuanlvInputQuery("app launchpad", "app")
	cmd.Command = "launchpad"
	assert.False(t, isBareKeywordQuery(cmd, ordinary))

	// selection queries stay immediate
	sel := kuankuanlvInputQuery("uuid", "uuid")
	sel.Type = QueryTypeSelection
	assert.False(t, isBareKeywordQuery(sel, ordinary))

	// nil plugin guard
	assert.False(t, isBareKeywordQuery(kuankuanlvInputQuery("uuid", "uuid"), nil))
}

// TestKuankuanlvPluginKeywordDetection verifies kw-level MRU identification:
// only a registered non-wildcard keyword matches.
func TestKuankuanlvPluginKeywordDetection(t *testing.T) {
	m := &Manager{}
	inst := newKuankuanlvInstance("test.kw", []string{"uuid", "*"}, nil, "")
	assert.True(t, m.isPluginKeyword(inst, "uuid"))
	assert.False(t, m.isPluginKeyword(inst, "*"))
	assert.False(t, m.isPluginKeyword(inst, "uu"))
	assert.False(t, m.isPluginKeyword(nil, "uuid"))
	assert.False(t, m.isPluginKeyword(inst, ""))
}

// TestKuankuanlvCandidateRowShape verifies the core-owned candidate row:
// stable identity, candidate chip, default confirm action that does not hide
// the window, and the ScoreKey feeding self-learning.
func TestKuankuanlvCandidateRowShape(t *testing.T) {
	kuankuanlvAdmissionLogger()
	m := &Manager{}
	ctx := context.Background()
	inst := newKuankuanlvInstance("test.shape", []string{"uuid"}, nil, "")

	row := m.newKeywordCandidateResult(ctx, inst, "UUID Plugin", "uuid", 42)
	require.Len(t, row.Actions, 1)
	assert.Equal(t, "kkc_test.shape_uuid", row.Id)
	assert.Equal(t, "uuid", row.Title)
	assert.Equal(t, "UUID Plugin", row.SubTitle)
	assert.Equal(t, "UUID Plugin", row.Group)
	assert.Equal(t, kuankuanlvCandidateScoreKeyPrefix+"test.shape:uuid", row.ScoreKey)
	assert.Equal(t, int64(42), row.Score)

	require.Len(t, row.TitleTags, 1)
	assert.Equal(t, QueryResultTitleTagKindKuankuanlvCandidate, row.TitleTags[0].Kind)

	action := row.Actions[0]
	assert.Equal(t, kuankuanlvCandidateConfirmActionID, action.Id)
	assert.True(t, action.IsDefault)
	assert.True(t, action.PreventHideAfterAction)
	assert.NotNil(t, action.Action, "confirm callback must be wired")
}

// TestKuankuanlvCandidatePrefixMatching verifies the pure Phase 1 candidate
// selector: only non-wildcard keywords starting with the typed prefix are
// emitted, matching is case-insensitive, and "*" is never a candidate.
func TestKuankuanlvCandidatePrefixMatching(t *testing.T) {
	assert.True(t, kuankuanlvCandidateKeywordMatches("uuid", "uu"))
	assert.True(t, kuankuanlvCandidateKeywordMatches("uuid", "uuid"))
	assert.True(t, kuankuanlvCandidateKeywordMatches("Boss", "bos"))
	assert.False(t, kuankuanlvCandidateKeywordMatches("*", "uu"))
	assert.False(t, kuankuanlvCandidateKeywordMatches("uuidx", "uuidy"))
	assert.False(t, kuankuanlvCandidateKeywordMatches("agent", "aa"))
	assert.False(t, kuankuanlvCandidateKeywordMatches("", "u"))
}
