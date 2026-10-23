package adapters

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"path/filepath"
	"strings"
)

func carryOwnership(before *domain.TargetBinding, p *Plan) error {
	if before == nil {
		return nil
	}
	docs := map[string]document{}
	for _, c := range p.Changes {
		name := filepath.Base(c.Path)
		var d document
		var err error
		switch name {
		case "config.toml":
			d, err = parseTOML(c.After)
		case "models.json":
			d, err = parseJSON(c.After, "models")
		case "auth.json", "settings.json":
			d, err = parseJSON(c.After, "strict")
		default:
			continue
		}
		if err != nil {
			return err
		}
		docs[name] = d
	}
	for key, want := range before.ExpectedFields {
		if strings.HasPrefix(key, "baseline:") || strings.HasPrefix(key, "meta:") {
			continue
		}
		if _, ok := p.Binding.ExpectedFields[key]; ok {
			continue
		}
		name, path, ok := strings.Cut(key, ":")
		if !ok {
			continue
		}
		d := docs[name]
		if d == nil {
			continue
		}
		v, exists := d.get(splitPath(path))
		if expectedEqual(v, exists, want) {
			p.Binding.ExpectedFields[key] = want
			if baseline, ok := before.ExpectedFields["baseline:"+key]; ok {
				p.Binding.ExpectedFields["baseline:"+key] = baseline
			}
		}
	}
	if _, ok := p.Binding.ExpectedFields["meta:catalog_hash"]; !ok {
		if value, ok := before.ExpectedFields["meta:catalog_hash"]; ok {
			p.Binding.ExpectedFields["meta:catalog_hash"] = value
			for _, path := range before.ManagedPaths {
				if filepath.Base(path) == "xiaobai-model-catalog.json" {
					p.Binding.ManagedPaths = append(p.Binding.ManagedPaths, path)
				}
			}
		}
	}
	return nil
}
func removeOwnedCatalog(p *Plan, b domain.TargetBinding) error {
	want := b.ExpectedFields["meta:catalog_hash"]
	if want == "" {
		return nil
	}
	for _, path := range b.ManagedPaths {
		if filepath.Base(path) != "xiaobai-model-catalog.json" {
			continue
		}
		c, e := fileops.Snapshot(path, nil)
		if e != nil {
			return e
		}
		if c.Existed && fingerprint(string(c.Before)) == want {
			c.Delete = true
			appendChange(p, c)
		}
	}
	return nil
}
func hasTrackedFields(b domain.TargetBinding) bool {
	for key := range b.ExpectedFields {
		if !strings.HasPrefix(key, "baseline:") && !strings.HasPrefix(key, "meta:") {
			return true
		}
	}
	return false
}

// A route/key rewrite updates only the owned field, never adopts unrelated live edits.
func updateWrittenExpectation(b *domain.TargetBinding, e *editor, path []string) error {
	changedKey := e.name + ":" + joinPath(path)
	value, _ := e.doc.get(path)
	for key, want := range b.ExpectedFields {
		if key != changedKey && !strings.HasPrefix(changedKey, key+"/") {
			continue
		}
		ownedPath := splitPath(strings.TrimPrefix(key, e.name+":"))
		var expected any
		if strings.HasPrefix(want, "rawsha256:") {
			b.ExpectedFields[key] = "rawsha256:" + fingerprint(asString(value))
			continue
		}
		if strings.HasPrefix(want, "authsha256:") {
			b.ExpectedFields[key] = "authsha256:" + fingerprint(DecodeAuthLiteral(asString(value)))
			continue
		}
		if strings.HasPrefix(want, "sha256:") {
			before, err := parseJSON(e.change.Before, "strict")
			if err != nil {
				return err
			}
			live, exists := before.get(ownedPath)
			if !expectedEqual(live, exists, want) {
				continue
			}
			updated, _ := e.doc.get(ownedPath)
			raw, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			b.ExpectedFields[key] = "sha256:" + fingerprint(string(raw))
			continue
		}
		if key == changedKey {
			expected = value
		} else {
			if err := json.Unmarshal([]byte(want), &expected); err != nil {
				return errors.New("invalid_binding: malformed expected field")
			}
			current := expected
			remaining := path[len(ownedPath):]
			for i, part := range remaining {
				obj, ok := current.(map[string]any)
				if !ok {
					return errors.New("invalid_binding: malformed expected object")
				}
				if i == len(remaining)-1 {
					obj[part] = value
				} else {
					current = obj[part]
				}
			}
		}
		raw, err := json.Marshal(expected)
		if err != nil {
			return err
		}
		s := string(raw)
		if e.name == "auth.json" || isSecret(ownedPath) {
			s = "sha256:" + fingerprint(s)
		}
		b.ExpectedFields[key] = s
	}
	return nil
}
