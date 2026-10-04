package launcher

import (
	"sort"
	"strings"

	"wox/plugin"
)

// registeredTriggerKeyword is one installed plugin's effective trigger keyword,
// already merged with the user's keyword overrides.
type registeredTriggerKeyword struct {
	keyword  string
	pluginID string
}

// keywordCompletionCandidate finishes a typed keyword token to one complete
// trigger keyword. The keyword prefix must be a strict prefix of a registered
// keyword; wildcard ("*") keywords are never completed. Ties break on the plugin
// that currently owns the running query, then shortest keyword, then
// alphabetically, so behavior is deterministic and unit-testable.
//
// It deliberately stays decoupled from parameter hints: it only completes the
// keyword text itself and never appends a trailing space (the user types the
// space, keeping keyword-only queries semantically correct).
func keywordCompletionCandidate(text string, keywords []registeredTriggerKeyword, queryPluginID string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	owned := strings.TrimSpace(queryPluginID)
	var candidates []registeredTriggerKeyword
	for _, item := range keywords {
		keyword := strings.TrimSpace(item.keyword)
		if keyword == "" || keyword == "*" {
			continue
		}
		if strings.HasPrefix(keyword, text) && keyword != text {
			candidates = append(candidates, registeredTriggerKeyword{keyword: keyword, pluginID: item.pluginID})
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		iOwned := candidates[i].pluginID == owned && owned != ""
		jOwned := candidates[j].pluginID == owned && owned != ""
		if iOwned != jOwned {
			return iOwned
		}
		if len(candidates[i].keyword) != len(candidates[j].keyword) {
			return len(candidates[i].keyword) < len(candidates[j].keyword)
		}
		return candidates[i].keyword < candidates[j].keyword
	})
	return candidates[0].keyword
}

// collectRegisteredTriggerKeywords walks the live plugin manager for effective
// trigger keywords of enabled plugins. Disabled plugins are skipped because
// their keywords can never be routed.
func collectRegisteredTriggerKeywords() []registeredTriggerKeyword {
	instances := plugin.GetPluginManager().GetPluginInstances()
	keywords := make([]registeredTriggerKeyword, 0, len(instances)*2)
	for _, instance := range instances {
		if instance == nil || instance.Setting == nil {
			continue
		}
		if instance.Setting.Disabled.Get() {
			continue
		}
		pluginID := strings.TrimSpace(instance.Metadata.Id)
		for _, keyword := range instance.GetTriggerKeywords() {
			keyword = strings.TrimSpace(keyword)
			if keyword == "" {
				continue
			}
			keywords = append(keywords, registeredTriggerKeyword{keyword: keyword, pluginID: pluginID})
		}
	}
	return keywords
}

// completeKeywordTab finishes the current keyword token to a complete trigger
// keyword. It runs after completePathTab so path completion is unchanged, and
// returns false (leaving the existing hint/reject flow to run) whenever the
// input is a path, already in parameter mode (contains whitespace), or has no
// matching registered keyword.
func (a *App) completeKeywordTab() bool {
	text := a.editor.State().Text
	if text == "" || isPathInput(text) || strings.ContainsAny(text, " \t") {
		return false
	}
	completion := keywordCompletionCandidate(text, collectRegisteredTriggerKeywords(), a.queryContext.PluginID)
	if completion == "" || completion == text {
		return false
	}
	a.rememberQueryHint()
	a.editor.SetText(completion, false)
	a.applyQueryTextChangeLocked(completion)
	a.reconcileSelectedPreview()
	_ = a.window.Invalidate()
	_ = a.sendCurrentQuery()
	return true
}
