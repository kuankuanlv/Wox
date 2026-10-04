package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"wox/common"
	"wox/setting"
	"wox/setting/definition"
)

type Instance struct {
	imageMu          sync.RWMutex
	imageCacheClosed bool

	Plugin               Plugin                 // plugin implementation
	API                  API                    // APIs exposed to plugin
	Metadata             Metadata               // metadata parsed from plugin.json
	IsSystemPlugin       bool                   // is system plugin, see `plugin.md` for more detail
	RuntimeLoaded        bool                   // host runtime has loaded this plugin
	Initialized          bool                   // plugin Init has run and runtime callbacks may be registered
	IsDevPlugin          bool                   // plugins loaded from `local plugin directories` which defined in wpm settings
	DevPluginDirectory   string                 // absolute path to dev plugin directory defined in wpm settings
	PluginDirectory      string                 // absolute path to plugin directory
	Host                 Host                   // plugin host to run this plugin
	Setting              *setting.PluginSetting // setting for this plugin
	RuntimeQueryCommands []MetadataCommand      // query commands registered at runtime

	runtimeTriggerKeywords   []string
	runtimeTriggerOptions    map[string]RegisterTriggerKeywordOption
	runtimeTriggerKeywordsMu sync.RWMutex

	// kuankuanlv runtime input filter: a runtime-registered admission filter for
	// wildcard ("*") plugins overrides the static metadata filter while set.
	runtimeInputFilter      *MetadataInputFilter
	inputFilterRegex        *regexp.Regexp // lazily compiled cache of the effective regex filter
	inputFilterRegexPattern string         // pattern the cache was compiled from; "" means none
	inputFilterMu           sync.RWMutex

	DynamicSettingCallbacks []func(ctx context.Context, key string) definition.PluginSettingDefinitionItem // dynamic setting callbacks
	SettingChangeCallbacks  []func(ctx context.Context, key string, value string)
	DeepLinkCallbacks       []func(ctx context.Context, arguments map[string]string)
	UnloadCallbacks         []func(ctx context.Context)
	MRURestoreCallbacks     []func(ctx context.Context, mruData MRUData) (*QueryResult, error) // MRU restore callbacks
	pluginTools             map[string]*pluginToolRegistration
	pluginToolsMu           sync.RWMutex
	// Unload closes admission before cancelling and draining active handlers.
	pluginToolsStopping       bool
	pluginToolCalls           map[*pluginToolActiveCall]struct{}
	EnterPluginQueryCallbacks []func(ctx context.Context)
	LeavePluginQueryCallbacks []func(ctx context.Context)
	DragOutCallbacks          []func(ctx context.Context, event DragOutEvent)

	// for measure performance
	LoadStartTimestamp    int64
	LoadFinishedTimestamp int64
	InitStartTimestamp    int64
	InitFinishedTimestamp int64

	// InitError is set when Init finishes unsuccessfully. Waiters distinguish
	// "still initializing" from "init completed with an error" through initDone.
	InitError       error
	initDone        chan struct{}
	initStateMu     sync.Mutex
	initLifecycleMu sync.Mutex
}

// beginInitCycle opens a new wait channel so later WaitInit calls block on this
// enable/load attempt instead of observing a previous finished cycle.
func (i *Instance) beginInitCycle() {
	if i == nil {
		return
	}
	i.pluginToolsMu.Lock()
	i.pluginToolsStopping = false
	i.pluginToolsMu.Unlock()
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	i.Initialized = false
	i.InitError = nil
	i.initDone = make(chan struct{})
}

// finishInit publishes the result before waking every waiter for this cycle.
func (i *Instance) finishInit(initialized bool, err error) {
	if i == nil {
		return
	}
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	i.Initialized = initialized
	i.InitError = err
	if i.initDone == nil {
		return
	}
	select {
	case <-i.initDone:
	default:
		close(i.initDone)
	}
}

func (i *Instance) resetInitState() {
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	i.Initialized = false
	i.InitError = nil
}

func (i *Instance) initStatus() (bool, error) {
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	return i.Initialized, i.InitError
}

// initCycleFinished reports whether the current cycle has notified its waiters.
func (i *Instance) initCycleFinished() bool {
	if i == nil {
		return true
	}
	i.initStateMu.Lock()
	defer i.initStateMu.Unlock()
	if i.initDone == nil {
		return true
	}
	select {
	case <-i.initDone:
		return true
	default:
		return false
	}
}

// WaitInit blocks until this instance's current init cycle finishes or ctx ends.
func (i *Instance) WaitInit(ctx context.Context) error {
	if i == nil {
		return fmt.Errorf("plugin instance is nil")
	}
	i.initStateMu.Lock()
	initDone := i.initDone
	i.initStateMu.Unlock()
	if initDone == nil {
		return fmt.Errorf("plugin init has not started")
	}
	select {
	case <-initDone:
		i.initStateMu.Lock()
		defer i.initStateMu.Unlock()
		return i.InitError
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for plugin init: %w", ctx.Err())
	}
}

func (i *Instance) translateMetadataText(ctx context.Context, text common.I18nString) string {
	return i.Metadata.translate(ctx, text)
}

func (i *Instance) TranslateMetadataText(ctx context.Context, text common.I18nString) string {
	return i.translateMetadataText(ctx, text)
}

func (i *Instance) GetName(ctx context.Context) string {
	return i.Metadata.GetName(ctx)
}

func (i *Instance) GetDescription(ctx context.Context) string {
	return i.Metadata.GetDescription(ctx)
}

// trigger keywords to trigger this plugin. Maybe user defined or pre-defined in plugin.json
func (i *Instance) GetTriggerKeywords() []string {
	keywords := i.Metadata.TriggerKeywords
	if i.Setting != nil && i.Setting.TriggerKeywords != nil {
		userDefinedKeywords := i.Setting.TriggerKeywords.Get()
		if len(userDefinedKeywords) > 0 {
			keywords = userDefinedKeywords
		}
	}

	i.runtimeTriggerKeywordsMu.RLock()
	defer i.runtimeTriggerKeywordsMu.RUnlock()
	if len(i.runtimeTriggerKeywords) == 0 {
		return keywords
	}
	return append(append([]string(nil), keywords...), i.runtimeTriggerKeywords...)
}

// setRuntimeTriggerKeywords replaces runtime registrations without persisting them as user overrides.
func (i *Instance) setRuntimeTriggerKeywords(keywords []string) {
	i.runtimeTriggerKeywordsMu.Lock()
	defer i.runtimeTriggerKeywordsMu.Unlock()
	i.runtimeTriggerKeywords = append([]string(nil), keywords...)
	i.runtimeTriggerOptions = nil
}

// unregisterTriggerKeyword leaves metadata and user-configured keywords untouched.
func (i *Instance) unregisterTriggerKeyword(keyword string) {
	i.runtimeTriggerKeywordsMu.Lock()
	defer i.runtimeTriggerKeywordsMu.Unlock()
	i.runtimeTriggerKeywords = slices.DeleteFunc(i.runtimeTriggerKeywords, func(value string) bool { return value == keyword })
	delete(i.runtimeTriggerOptions, keyword)
}

// triggerQueryHint returns an isolated runtime override or the static keyword template.
func (i *Instance) triggerQueryHint(keyword string) *common.QueryHint {
	i.runtimeTriggerKeywordsMu.RLock()
	defer i.runtimeTriggerKeywordsMu.RUnlock()
	if option, exists := i.runtimeTriggerOptions[keyword]; exists {
		return option.QueryHint.Clone()
	}
	return i.Metadata.TriggerQueryHints[keyword].Clone()
}

// PrimaryTriggerKeyword returns the first non-global ("*") trigger keyword.
// Scoped queries must not use GetTriggerKeywords()[0] because "*" often comes first.
func (i *Instance) PrimaryTriggerKeyword() string {
	for _, keyword := range i.GetTriggerKeywords() {
		if keyword != "" && keyword != "*" {
			return keyword
		}
	}
	return ""
}

// query commands to query this plugin. Commands come from plugin metadata and runtime registration only.
func (i *Instance) GetQueryCommands() []MetadataCommand {
	commands := make([]MetadataCommand, 0, len(i.Metadata.Commands)+len(i.RuntimeQueryCommands))
	seen := make(map[string]struct{}, len(i.Metadata.Commands)+len(i.RuntimeQueryCommands))
	translateCtx := context.Background()

	appendCommand := func(command MetadataCommand) {
		if command.Command == "" {
			return
		}
		if _, exists := seen[command.Command]; exists {
			return
		}
		seen[command.Command] = struct{}{}
		command.Description = common.I18nString(i.translateMetadataText(translateCtx, command.Description))
		commands = append(commands, command)
	}

	for _, command := range i.Metadata.Commands {
		appendCommand(command)
	}

	for _, command := range i.RuntimeQueryCommands {
		appendCommand(command)
	}

	return commands
}

// cloneMetadataInputFilter returns an isolated copy so runtime registrations can
// never alias the caller's or the metadata's slices.
func cloneMetadataInputFilter(filter *MetadataInputFilter) *MetadataInputFilter {
	if filter == nil {
		return nil
	}
	cloned := *filter
	cloned.Items = append([]string(nil), filter.Items...)
	return &cloned
}

// Kuankuanlv user-override setting keys. The settings UI persists user edits
// through the existing plugin setting KV channel (wox.db-backed); core reads
// them back so a user edit wins over the runtime/metadata filter. The string
// values must stay byte-identical to wox/ui/dto/plugin_dto.go; this package
// cannot import wox/ui/dto (it would be an import cycle). The Actions override
// key is intentionally not consumed here: the UI action projection already reads
// it directly through instance.API.GetSetting.
const (
	kuankuanlvSettingKeyInputFilterItems   = "KuankuanlvInputFilterItems"
	kuankuanlvSettingKeyInputFilterPattern = "KuankuanlvInputFilterPattern"
)

// GetEffectiveInputFilter returns the input filter actually used for wildcard
// ("*") admission, in priority order:
//  1. user setting override (written from the settings UI, read live so a GUI
//     save takes effect on the next query without a plugin reload),
//  2. runtime-registered filter (Instance.RegisterInputFilter),
//  3. static metadata-declared filter.
//
// A nil result means no filter is declared: wildcard ("*") plugins then keep
// the upstream behavior of participating in every eligible query.
func (i *Instance) GetEffectiveInputFilter() *MetadataInputFilter {
	if i == nil {
		return nil
	}
	i.inputFilterMu.RLock()
	defer i.inputFilterMu.RUnlock()
	if override := i.kuankuanlvSettingFilterOverride(); override != nil {
		return cloneMetadataInputFilter(override)
	}
	if i.runtimeInputFilter != nil {
		return cloneMetadataInputFilter(i.runtimeInputFilter)
	}
	return cloneMetadataInputFilter(i.Metadata.KuankuanlvInputFilter)
}

// kuankuanlvSettingFilterOverride rebuilds the user-persisted input filter
// override from the plugin setting KV, when one exists. The override's mode is
// taken from the author-declared metadata filter because the UI only renders
// the matching editor (list table vs regex textbox) for a declared mode:
// list mode reads KuankuanlvInputFilterItems (a JSON string array), regex mode
// reads KuankuanlvInputFilterPattern (a plain regex string). A nil return means
// the user saved no override (or it is malformed), so admission falls through
// to runtime registration and metadata. Callers must hold inputFilterMu.
func (i *Instance) kuankuanlvSettingFilterOverride() *MetadataInputFilter {
	if i == nil || i.API == nil || i.Setting == nil {
		return nil
	}
	base := i.Metadata.KuankuanlvInputFilter
	if base == nil {
		return nil
	}
	ctx := context.Background()
	switch {
	case base.IsList():
		raw := strings.TrimSpace(i.API.GetSetting(ctx, kuankuanlvSettingKeyInputFilterItems))
		if raw == "" {
			return nil
		}
		var items []string
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			logger.Warn(ctx, fmt.Sprintf("plugin <%s>: malformed kuankuanlv input filter items override %q, ignoring: %s",
				i.GetName(ctx), raw, err.Error()))
			return nil
		}
		if len(items) == 0 {
			return nil
		}
		return &MetadataInputFilter{Mode: "list", Items: items}
	case base.IsRegex():
		pattern := strings.TrimSpace(i.API.GetSetting(ctx, kuankuanlvSettingKeyInputFilterPattern))
		if pattern == "" {
			return nil
		}
		if _, err := regexp.Compile(pattern); err != nil {
			logger.Warn(ctx, fmt.Sprintf("plugin <%s>: invalid kuankuanlv input filter pattern override %q, ignoring: %s",
				i.GetName(ctx), pattern, err.Error()))
			return nil
		}
		return &MetadataInputFilter{Mode: "regex", Pattern: pattern}
	}
	return nil
}

// RegisterInputFilter stores a runtime input filter, replacing any previous
// runtime registration. The static metadata filter stays untouched. Passing nil
// clears the runtime override; the regex cache is invalidated either way.
func (i *Instance) RegisterInputFilter(filter *MetadataInputFilter) {
	if i == nil {
		return
	}
	i.inputFilterMu.Lock()
	defer i.inputFilterMu.Unlock()
	i.runtimeInputFilter = cloneMetadataInputFilter(filter)
	i.inputFilterRegex = nil
	i.inputFilterRegexPattern = ""
}

// UnregisterInputFilter clears the runtime input filter override, falling back
// to the static metadata filter.
func (i *Instance) UnregisterInputFilter() {
	i.RegisterInputFilter(nil)
}

// kuankuanlvInputFilterAdmits reports whether rawQuery passes the effective
// input filter admission. A nil effective filter admits everything (upstream
// behavior); an unknown mode or an uncompilable runtime regex fails open with a
// warning, mirroring the metadata validator that drops invalid filters.
func (i *Instance) kuankuanlvInputFilterAdmits(rawQuery string) bool {
	if i == nil {
		return true
	}
	filter := i.GetEffectiveInputFilter()
	if filter == nil {
		return true
	}

	if filter.IsList() {
		for _, item := range filter.Items {
			if item == "" {
				continue
			}
			// Bidirectional prefix match, case-insensitive: both "user typing
			// into the middle of an item" and "item being a longer prefix of
			// what the user has typed so far" must admit the plugin.
			if hasPrefixFold(rawQuery, item) || hasPrefixFold(item, rawQuery) {
				return true
			}
		}
		return false
	}

	if filter.IsRegex() {
		regex, compiled := i.kuankuanlvInputFilterRegex(filter.Pattern)
		if !compiled {
			// The metadata validator already dropped invalid static patterns;
			// a runtime-registered pattern that cannot compile fails open so a
			// bad host registration never silently silences the plugin.
			logger.Warn(context.Background(), fmt.Sprintf(
				"plugin <%s>: runtime input filter pattern %q failed to compile, admitting all input", i.GetName(context.Background()), filter.Pattern))
			return true
		}
		return regex.MatchString(rawQuery)
	}

	return true
}

// kuankuanlvInputFilterRegex lazily compiles and caches the regexp for the
// effective filter pattern. The cache is invalidated by RegisterInputFilter,
// so it never outlives the pattern it was compiled from.
func (i *Instance) kuankuanlvInputFilterRegex(pattern string) (*regexp.Regexp, bool) {
	i.inputFilterMu.RLock()
	if i.inputFilterRegex != nil && i.inputFilterRegexPattern == pattern {
		regex := i.inputFilterRegex
		i.inputFilterMu.RUnlock()
		return regex, true
	}
	i.inputFilterMu.RUnlock()

	i.inputFilterMu.Lock()
	defer i.inputFilterMu.Unlock()
	if i.inputFilterRegex != nil && i.inputFilterRegexPattern == pattern {
		return i.inputFilterRegex, true
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		// Keep the cache clean: a failed compile is retried on the next query
		// (harmless, and self-heals if the plugin re-registers a valid pattern).
		i.inputFilterRegex = nil
		i.inputFilterRegexPattern = ""
		return nil, false
	}
	i.inputFilterRegex = compiled
	i.inputFilterRegexPattern = pattern
	return compiled, true
}

func hasPrefixFold(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

func (i *Instance) String() string {
	return i.GetName(context.Background())
}
