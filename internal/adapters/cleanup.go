package adapters

import (
	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"strings"
)

func cleanupRevertedProviders(b domain.TargetBinding, docs map[string]*editor) error {
	if b.ProviderID == nil {
		return nil
	}
	id := *b.ProviderID
	if b.Target == domain.Codex {
		if e := docs["config.toml"]; e != nil {
			d := e.doc.(*tomlDoc)
			parent := d.doc.First("model_providers", id)
			if parent != nil {
				empty := false
				if parent.KeyValue != nil {
					v, ok := parent.Value.X.(parser.Inline)
					empty = ok && len(v) == 0
				} else {
					empty = true
					parent.Section.Scan(func(k parser.Key, entry *tomledit.Entry) bool {
						if entry.KeyValue != nil {
							empty = false
						}
						return true
					})
				}
				if empty {
					parent.Remove()
				}
			}
		}
		return nil
	}
	if b.ExpectedFields["meta:legacy"] != "true" || (b.Target != domain.Pi && b.Target != domain.Prime) {
		return nil
	}
	for _, name := range []string{"models.json", "auth.json"} {
		e := docs[name]
		if e == nil {
			continue
		}
		original, err := parseJSON(e.change.Before, map[bool]string{true: "models", false: "strict"}[name == "models.json"])
		if err != nil {
			return err
		}
		matches := true
		prefix := name + ":"
		count := 0
		for key, want := range b.ExpectedFields {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			path := splitPath(strings.TrimPrefix(key, prefix))
			v, exists := original.get(path)
			matches = matches && expectedEqual(v, exists, want)
			count++
		}
		if matches && count > 0 {
			path := []string{id}
			if name == "models.json" {
				path = []string{"providers", id}
			}
			if err = e.doc.set(path, nil, true); err != nil {
				return err
			}
		}
	}
	return nil
}
