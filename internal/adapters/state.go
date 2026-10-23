package adapters

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"path/filepath"
	"sort"
	"strings"
)

type State struct {
	Status      domain.ApplyStatus `json:"status"`
	Fields      map[string]string  `json:"fields"`
	Differences []string           `json:"differences"`
}

func expectedEqual(value any, exists bool, want string) bool {
	if want == missing {
		return !exists
	}
	if strings.HasPrefix(want, "rawsha256:") {
		return exists && "rawsha256:"+fingerprint(asString(value)) == want
	}
	if strings.HasPrefix(want, "authsha256:") {
		return exists && "authsha256:"+fingerprint(DecodeAuthLiteral(asString(value))) == want
	}
	if !exists {
		return false
	}
	b, e := json.Marshal(value)
	if e != nil {
		return false
	}
	got := string(b)
	if strings.HasPrefix(want, "sha256:") {
		return "sha256:"+fingerprint(got) == want
	}
	return got == want
}
func bindingDocs(b domain.TargetBinding) (map[string]*editor, error) {
	p := Plan{}
	docs := map[string]*editor{}
	for _, path := range b.ManagedPaths {
		name := filepath.Base(path)
		mode := "strict"
		switch name {
		case "config.toml":
			mode = "toml"
		case "models.json":
			mode = "models"
		case "auth.json", "settings.json":
		default:
			continue
		}
		e, err := openEditor(&p, path, mode)
		if err != nil {
			return nil, err
		}
		docs[name] = e
	}
	return docs, nil
}
func ReadState(b *domain.TargetBinding) (State, error) {
	s := State{Status: domain.NotApplied, Fields: map[string]string{}}
	if b == nil {
		return s, nil
	}
	normalized := normalizeBinding(*b)
	b = &normalized
	docs, e := bindingDocs(*b)
	if e != nil {
		return s, e
	}
	s.Status = domain.Applied
	if b.Orphan {
		s.Status = domain.Orphan
	}
	for key, want := range b.ExpectedFields {
		if strings.HasPrefix(key, "baseline:") || key == "meta:legacy" || key == "meta:env_mode" || key == "meta:catalog_hash" || key == "meta:capability_source" || key == "meta:auth_source" {
			continue
		}
		name, path, ok := strings.Cut(key, ":")
		if !ok {
			s.Differences = append(s.Differences, "unrecognized:"+key)
			continue
		}
		doc := docs[name]
		if doc == nil {
			s.Differences = append(s.Differences, key)
			continue
		}
		parts := splitPath(path)
		value, exists := doc.doc.get(parts)
		if !expectedEqual(value, exists, want) {
			s.Differences = append(s.Differences, key)
		}
		if exists {
			if name == "auth.json" || isSecret(parts) {
				s.Fields[key] = "[redacted]"
			} else {
				raw, _ := json.Marshal(value)
				s.Fields[key] = string(raw)
			}
		}
	}
	if !hasTrackedFields(*b) {
		s.Differences = append(s.Differences, "unrecognized_binding_schema")
	}
	if b.Target == domain.Codex {
		path := envPath(*b)
		if path == "" {
			s.Differences = append(s.Differences, "codex.env")
		} else {
			raw, _, err := fileops.Read(path)
			if err != nil {
				return s, err
			}
			for _, key := range b.ManagedEnvKeys {
				v, exists, err := envValue(raw, key)
				if err != nil {
					return s, err
				}
				if !exists || fingerprint(v) != b.KeyFingerprint {
					s.Differences = append(s.Differences, "codex.env:"+key)
				}
			}
		}
	}
	if b.Target == domain.Pi || b.Target == domain.Prime {
		if b.ProviderID != nil {
			doc := docs["models.json"]
			found := false
			if doc != nil {
				models, _ := doc.doc.get([]string{"providers", *b.ProviderID, "models"})
				if list, ok := models.([]any); ok {
					for _, m := range list {
						if obj, ok := m.(map[string]any); ok && obj["id"] == b.ModelID {
							found = true
						}
					}
				}
			}
			if !found {
				s.Differences = append(s.Differences, "models.json:selected_model")
			}
		}
	}
	if len(s.Differences) > 0 && !b.Orphan {
		s.Status = domain.Stale
	}
	sort.Strings(s.Differences)
	return s, nil
}

// Revert restores only fields whose live value still equals our last write.
func Revert(b domain.TargetBinding) (Plan, error) {
	b = normalizeBinding(b)
	p := Plan{Target: b.Target, Binding: b}
	if b.SiteID != nil {
		p.SiteID = *b.SiteID
	}
	docs, e := bindingDocs(b)
	if e != nil {
		return p, e
	}
	keys := []string{}
	for k := range b.ExpectedFields {
		if !strings.HasPrefix(k, "baseline:") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		name, path, ok := strings.Cut(key, ":")
		if !ok {
			continue
		}
		doc := docs[name]
		if doc == nil {
			continue
		}
		parts := splitPath(path)
		live, exists := doc.doc.get(parts)
		if !expectedEqual(live, exists, b.ExpectedFields[key]) {
			continue
		}
		baseline := b.ExpectedFields["baseline:"+key]
		remove := baseline == "" || baseline == missing
		var value any
		if !remove {
			if e = json.Unmarshal([]byte(baseline), &value); e != nil {
				return p, errors.New("invalid_binding: malformed baseline")
			}
		}
		if e = doc.doc.set(parts, value, remove); e != nil {
			return p, e
		}
	}
	if e = cleanupRevertedProviders(b, docs); e != nil {
		return p, e
	}
	for _, doc := range docs {
		if doc.change.Existed {
			doc.change.After = doc.doc.bytes()
			p.Changes = append(p.Changes, doc.change)
		}
	}
	if b.Target == domain.Codex {
		if e = removeOwnedCatalog(&p, b); e != nil {
			return p, e
		}
		if e = revertEnv(&p, b); e != nil {
			return p, e
		}
		if b.ExpectedFields["meta:env_mode"] == "user_env" && len(b.ManagedEnvKeys) > 0 {
			p.Injection = &envinject.Plan{Mode: "user_env", UserKey: b.ManagedEnvKeys[0], UserRemove: true, UserExpectedFingerprint: b.KeyFingerprint}
		}
	}
	return p, nil
}
