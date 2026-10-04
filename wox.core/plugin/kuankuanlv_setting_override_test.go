package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"wox/setting"
)

// kuankuanlvSettingStubAPI stands in for the plugin-facing API so a unit test
// can feed GetSetting values without booting the real setting manager. Embedding
// the API interface leaves every method nil; only GetSetting is overridden,
// which is the only method the override path calls.
type kuankuanlvSettingStubAPI struct {
	API
	settings map[string]string
}

func (s kuankuanlvSettingStubAPI) GetSetting(_ context.Context, key string) string {
	return s.settings[key]
}

// TestKuankuanlvSettingInputFilterOverrideWins verifies the user-persisted
// setting override takes priority over both the runtime-registered filter and
// the metadata filter (setting > runtime > metadata), and that clearing the
// override falls back down the chain.
func TestKuankuanlvSettingInputFilterOverrideWins(t *testing.T) {
	kuankuanlvAdmissionLogger()
	m := &Manager{}
	ctx := context.Background()

	// metadata declares list item "meta"; the user overrides items to "usercmd".
	inst := newKuankuanlvInstance("test.override", []string{"*"},
		&MetadataInputFilter{Mode: "list", Items: []string{"meta"}}, "")
	inst.Setting = &setting.PluginSetting{}
	inst.API = kuankuanlvSettingStubAPI{settings: map[string]string{
		kuankuanlvSettingKeyInputFilterItems: `["usercmd"]`,
	}}

	effective := inst.GetEffectiveInputFilter()
	require.NotNil(t, effective)
	assert.Equal(t, []string{"usercmd"}, effective.Items)
	assert.True(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("usercmd", "")), "override item admitted")
	assert.False(t, m.canOperateQuery(ctx, inst, kuankuanlvInputQuery("meta", "")), "setting override shadows metadata")

	// A runtime registration must NOT beat the setting override.
	inst.RegisterInputFilter(&MetadataInputFilter{Mode: "list", Items: []string{"rt"}})
	assert.Equal(t, []string{"usercmd"}, inst.GetEffectiveInputFilter().Items, "setting override > runtime")

	// Clear the override: fall back to runtime registration, then metadata.
	inst.API = kuankuanlvSettingStubAPI{settings: map[string]string{}}
	assert.Equal(t, []string{"rt"}, inst.GetEffectiveInputFilter().Items, "no override -> runtime")
	inst.UnregisterInputFilter()
	assert.Equal(t, []string{"meta"}, inst.GetEffectiveInputFilter().Items, "no override/runtime -> metadata")
}

// TestKuankuanlvSettingRegexOverride verifies regex-mode override from the
// KuankuanlvInputFilterPattern setting key.
func TestKuankuanlvSettingRegexOverride(t *testing.T) {
	kuankuanlvAdmissionLogger()
	m := &Manager{}
	inst := newKuankuanlvInstance("test.override-regex", []string{"*"},
		&MetadataInputFilter{Mode: "regex", Pattern: "^meta$"}, "")
	inst.Setting = &setting.PluginSetting{}
	inst.API = kuankuanlvSettingStubAPI{settings: map[string]string{
		kuankuanlvSettingKeyInputFilterPattern: `^user$`,
	}}

	assert.True(t, m.canOperateQuery(context.Background(), inst, kuankuanlvInputQuery("user", "")))
	assert.False(t, m.canOperateQuery(context.Background(), inst, kuankuanlvInputQuery("meta", "")), "regex override shadows metadata")
}

// TestKuankuanlvSettingOverrideNoMetadataFilter: without a metadata-declared
// filter the override is ignored (the editor only renders for a declared mode),
// preserving upstream behavior for ordinary plugins.
func TestKuankuanlvSettingOverrideNoMetadataFilter(t *testing.T) {
	kuankuanlvAdmissionLogger()
	inst := newKuankuanlvInstance("test.override-none", []string{"*"}, nil, "")
	inst.Setting = &setting.PluginSetting{}
	inst.API = kuankuanlvSettingStubAPI{settings: map[string]string{
		kuankuanlvSettingKeyInputFilterItems: `["anything"]`,
	}}
	assert.Nil(t, inst.GetEffectiveInputFilter(), "no metadata filter -> no override applied")
}
