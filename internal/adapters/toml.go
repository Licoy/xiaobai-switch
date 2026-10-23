package adapters

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"strings"
)

type tomlDoc struct{ doc *tomledit.Document }

func parseTOML(b []byte) (*tomlDoc, error) {
	d, e := tomledit.Parse(bytes.NewReader(b))
	if e != nil {
		return nil, errors.New("invalid_config: invalid TOML syntax")
	}
	return &tomlDoc{d}, nil
}
func (t *tomlDoc) get(path []string) (any, bool) {
	e := t.doc.First(path...)
	if e == nil || e.KeyValue == nil {
		return nil, false
	}
	s := e.Value.String()
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		return v, true
	}
	return s, true
}
func (t *tomlDoc) set(path []string, value any, remove bool) error {
	old := t.doc.First(path...)
	if remove {
		if old != nil {
			old.Remove()
		}
		return nil
	}
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	v, e := parser.ParseValue(string(b))
	if e != nil {
		return errors.New("invalid_config: unsupported TOML value")
	}
	if old != nil {
		if old.KeyValue == nil {
			return errors.New("invalid_config: expected scalar TOML field")
		}
		v.Trailer = old.Value.Trailer
		old.Value = v
		return nil
	}
	for n := len(path) - 1; n > 0; n-- {
		parent := t.doc.First(path[:n]...)
		if parent == nil || parent.KeyValue == nil {
			continue
		}
		inline, ok := parent.Value.X.(parser.Inline)
		if !ok {
			return errors.New("invalid_config: parent TOML field must be a table")
		}
		parent.Value.X = append(inline, &parser.KeyValue{Name: parser.Key(path[n:]), Value: v})
		return nil
	}
	sectionPath := parser.Key(path[:len(path)-1])
	var section *tomledit.Section
	if len(sectionPath) == 0 {
		if t.doc.Global == nil {
			t.doc.Global = &tomledit.Section{}
		}
		section = t.doc.Global
	} else {
		for _, s := range t.doc.Sections {
			if s.TableName().Equals(sectionPath) {
				section = s
				break
			}
		}
		if section == nil {
			section = &tomledit.Section{Heading: &parser.Heading{Name: sectionPath}}
			t.doc.Sections = append(t.doc.Sections, section)
		}
	}
	section.Items = append(section.Items, &parser.KeyValue{Name: parser.Key{path[len(path)-1]}, Value: v})
	return nil
}
func (t *tomlDoc) bytes() []byte {
	var out bytes.Buffer
	_ = tomledit.Format(&out, t.doc)
	return out.Bytes()
}
func (t *tomlDoc) removeManagedProviders() {
	var entries []*tomledit.Entry
	t.doc.Scan(func(k parser.Key, e *tomledit.Entry) bool {
		if len(k) > 1 && k[0] == "model_providers" && strings.HasPrefix(k[1], "xiaobai_") {
			entries = append(entries, e)
		}
		return true
	})
	for i := len(entries) - 1; i >= 0; i-- {
		entries[i].Remove()
	}
}
func (t *tomlDoc) hasManagedProvider() bool {
	found := false
	t.doc.Scan(func(path parser.Key, _ *tomledit.Entry) bool {
		if len(path) > 1 && path[0] == "model_providers" && strings.HasPrefix(path[1], "xiaobai_") {
			found = true
			return false
		}
		return true
	})
	return found
}
