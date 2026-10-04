package dto

import (
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
)

type PluginDto struct {
	Id                 string
	Name               string
	NameEn             string
	Author             string
	Version            string
	MinWoxVersion      string
	Runtime            string
	Description        string
	DescriptionEn      string
	Icon               common.WoxImage
	Website            string
	Entry              string
	PluginDirectory    string // only available when plugin is installed
	ScreenshotUrls     []string
	TriggerKeywords    []string //User can add/update/delete trigger keywords
	Commands           []plugin.MetadataCommand
	SupportedOS        []string
	SettingDefinitions definition.PluginSettingDefinitions // only available when plugin is installed
	Setting            PluginSettingDto                    // only available when plugin is installed
	Features           []plugin.MetadataFeature            // only available when plugin is installed
	Glances            []plugin.MetadataGlance
	IsSystem           bool
	IsDev              bool
	IsInstalled        bool
	IsDisable          bool // only available when plugin is installed
	IsUpgradable       bool

	// Kuankuanlv fork extension fields. They are copied verbatim from the plugin
	// metadata by name (see convertPluginInstanceToDto) so the settings UI can
	// render the extension area without importing plugin-internal logic.
	KuankuanlvSchemaVersion      int                    `json:"kuankuanlv_schema_version"`
	KuankuanlvInputFilter        *plugin.MetadataInputFilter `json:"kuankuanlv_input_filter"`
	KuankuanlvParameterHint      string                 `json:"kuankuanlv_parameter_hint"`
	KuankuanlvInputDescription   string                 `json:"kuankuanlv_input_description"`
	KuankuanlvOutputDescription string                 `json:"kuankuanlv_output_description"`
	KuankuanlvActions            []plugin.MetadataAction `json:"kuankuanlv_actions"`
}

// Kuankuanlv GUI override setting keys. User edits made in the plugin settings
// extension area are persisted through the existing plugin setting KV channel
// (instance.API.SaveSetting / GetSetting, backed by wox.db); no new tables or
// direct database access are involved. Core is expected to read these keys back
// and apply runtime overrides (e.g. Instance.RegisterInputFilter).
const (
	KuankuanlvSettingInputFilterItems   = "KuankuanlvInputFilterItems"
	KuankuanlvSettingInputFilterPattern = "KuankuanlvInputFilterPattern"
	KuankuanlvSettingActions            = "KuankuanlvActions"
)

type PluginSettingDto struct {
	Disabled        bool
	TriggerKeywords []string
	Settings        map[string]string
}

type PluginQueryCommandDto struct {
	Command     string
	Description string
}
