package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSettingsUnknownFieldsAndDefaults(t *testing.T) {
	var v AppSettings
	e := json.Unmarshal([]byte(`{"primaryColor":"#1677ff","startInTray":true,"future":{"enabled":true},"launchAtLoginHidden":true}`), &v)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := v.Extra["launchAtLoginHidden"]; ok {
		t.Fatal("known omitempty field stored as extra")
	}
	v = NormalizeSettings(v)
	if v.PrimaryColor != "#0C76F8" || v.StartInTray || v.LaunchAtLoginHidden == nil || !*v.LaunchAtLoginHidden {
		t.Fatalf("normalization %+v", v)
	}
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), `"future":{"enabled":true}`) {
		t.Fatalf("lost unknown setting %s", b)
	}
	v.PrimaryColor = "#123456"
	if NormalizeSettings(v).PrimaryColor != "#123456" {
		t.Fatal("changed custom color")
	}
}
func TestProviderAndThinkingRules(t *testing.T) {
	if got := ProviderIDForSite("ABCDEF12-3456-9999"); got != "xiaobai_abcdef123456" {
		t.Fatal(got)
	}
	level := "max"
	mapping := "provider_max"
	w := ThinkingWrite{DefaultLevel: &level, Extended: ThinkingExtendedMap{Max: &mapping}, Models: map[string]ModelThinkingConfig{"a": {Reasoning: true}}}
	if e := ValidateThinking(w, Pi, OpenAICompatible, "a"); e != nil {
		t.Fatal(e)
	}
	if e := ValidateThinking(w, Prime, Anthropic, "a"); e == nil {
		t.Fatal("accepted Prime Anthropic max")
	}
	if e := ValidateThinking(w, Pi, Anthropic, "a"); e == nil {
		t.Fatal("accepted nonadaptive Pi Anthropic max")
	}
	w.Models["a"] = ModelThinkingConfig{Reasoning: true, ForceAdaptiveThinking: true}
	if e := ValidateThinking(w, Pi, Anthropic, "a"); e != nil {
		t.Fatal(e)
	}
}
