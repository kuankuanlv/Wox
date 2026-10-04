package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"wox/common"
	"wox/setting"
)

func TestMatchTriggerKeywordPrefix(t *testing.T) {
	cases := []struct {
		name     string
		keywords []string
		word     string
		want     bool
	}{
		{"exact", []string{"uuid", "tt", "fmt"}, "uuid", true},
		{"prefix", []string{"uuid", "update"}, "uu", true},
		{"prefix-shorter", []string{"update"}, "u", true},
		{"wildcard-excluded", []string{"*"}, "u", false},
		{"wildcard-with-others", []string{"*", "uuid"}, "uu", true},
		{"empty-word", []string{"uuid"}, "", false},
		{"no-match", []string{"fmt", "tt"}, "uuid", false},
		{"case-insensitive", []string{"Uuid"}, "uu", true},
	}
	for _, c := range cases {
		if got := matchTriggerKeyword(c.keywords, c.word); got != c.want {
			t.Errorf("%s: matchTriggerKeyword(%v, %q) = %v, want %v", c.name, c.keywords, c.word, got, c.want)
		}
	}
}

func TestCanOperateQueryWildcard(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{}
	app := testPluginInstance("ea2b6859-14bc-4c89-9c88-627da7379141", "*")
	folder := testPluginInstance("527ba64f-c8f5-4fc7-bb98-306f79d27f32", "*")
	uuid := testPluginInstance("plg-uuid", "uuid")
	manager.appendPluginInstance(uuid)

	// bare keyword query: any "*" plugin participates alongside the keyword
	// owner; a plugin decides itself whether to register "*"
	bare := Query{Type: QueryTypeInput, RawQuery: "uuid", TriggerKeyword: "uuid"}
	assert.True(t, manager.canOperateQuery(ctx, app, bare))
	assert.True(t, manager.canOperateQuery(ctx, folder, bare))
	assert.True(t, manager.canOperateQuery(ctx, uuid, bare))

	// prefix keyword query: the keyword plugin and "*" plugins both participate;
	// relevance is enforced per-plugin (e.g. app search matches display name +
	// bundle id only)
	prefix := Query{Type: QueryTypeInput, RawQuery: "uu", TriggerKeyword: "uu"}
	assert.True(t, manager.canOperateQuery(ctx, uuid, prefix))
	assert.True(t, manager.canOperateQuery(ctx, app, prefix))
	assert.True(t, manager.canOperateQuery(ctx, folder, prefix))

	// parameter query (space): keyword plugin runs, "*" plugins do not
	param := Query{Type: QueryTypeInput, RawQuery: "uuid v5", TriggerKeyword: "uuid", Search: "v5"}
	assert.True(t, manager.canOperateQuery(ctx, uuid, param))
	assert.False(t, manager.canOperateQuery(ctx, app, param))
	assert.False(t, manager.canOperateQuery(ctx, folder, param))

	// plain global query: "*" plugins participate, keyword-only plugins do not
	global := Query{Type: QueryTypeInput, RawQuery: "scottqian", Search: "scottqian"}
	assert.True(t, manager.canOperateQuery(ctx, app, global))
	assert.True(t, manager.canOperateQuery(ctx, folder, global))
	assert.False(t, manager.canOperateQuery(ctx, uuid, global))

	// scoped query routes to the pinned plugin directly
	scoped := Query{Type: QueryTypeInput, RawQuery: "uuid", TriggerKeyword: "uuid",
		Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: "plg-uuid"}}}}
	assert.True(t, manager.canOperateQuery(ctx, uuid, scoped))
	assert.False(t, manager.canOperateQuery(ctx, app, scoped))
}

func testPluginInstance(id string, keywords ...string) *Instance {
	return &Instance{
		Metadata: Metadata{Id: id, TriggerKeywords: keywords},
		Setting:  &setting.PluginSetting{},
	}
}

func TestParseBareKeywordPrefix(t *testing.T) {
	instances := []*Instance{
		testPluginInstance("plg-uuid", "uuid"),
		testPluginInstance("plg-folder", "*"),
	}
	q, owner := newQueryInputWithPlugins("uu", instances)
	assert.Equal(t, "uu", q.TriggerKeyword)
	assert.NotNil(t, owner)
	assert.Equal(t, "plg-uuid", owner.Metadata.Id)

	// non-keyword word keeps an empty trigger keyword
	q, owner = newQueryInputWithPlugins("scottqian", instances)
	assert.Equal(t, "", q.TriggerKeyword)
	assert.Nil(t, owner)
}
