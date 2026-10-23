package adapters

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/tailscale/hujson"
	"strings"
)

type jsonDoc struct{ value hujson.Value }

func parseJSON(b []byte, mode string) (*jsonDoc, error) {
	if len(b) == 0 {
		b = []byte("{}")
	}
	if mode == "strict" && !json.Valid(b) {
		return nil, errors.New("invalid_config: strict JSON required")
	}
	// HuJSON edits alias its input; keep file snapshots immutable.
	v, e := hujson.Parse(bytes.Clone(b))
	if e != nil {
		return nil, errors.New("invalid_config: invalid JSON syntax")
	}
	if _, ok := v.Value.(*hujson.Object); !ok {
		return nil, errors.New("invalid_config: JSON root must be object")
	}
	if mode == "models" {
		bad := false
		v.Range(func(n *hujson.Value) bool {
			if strings.Contains(string(n.BeforeExtra), "/*") || strings.Contains(string(n.AfterExtra), "/*") {
				bad = true
			}
			if o, ok := n.Value.(*hujson.Object); ok && strings.Contains(string(o.AfterExtra), "/*") {
				bad = true
			}
			if a, ok := n.Value.(*hujson.Array); ok && strings.Contains(string(a.AfterExtra), "/*") {
				bad = true
			}
			return true
		})
		if bad {
			return nil, errors.New("invalid_config: block comments are unsupported by target")
		}
	}
	return &jsonDoc{v}, nil
}
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
func (j *jsonDoc) get(path []string) (any, bool) {
	v := &j.value
	for _, k := range path {
		o, ok := v.Value.(*hujson.Object)
		if !ok {
			return nil, false
		}
		var next *hujson.Value
		for i := range o.Members {
			if o.Members[i].Name.Value.(hujson.Literal).String() == k {
				next = &o.Members[i].Value
				break
			}
		}
		if next == nil {
			return nil, false
		}
		v = next
	}
	c := v.Clone()
	c.Standardize()
	var x any
	if json.Unmarshal(c.Pack(), &x) != nil {
		return nil, false
	}
	return x, true
}
func (j *jsonDoc) set(path []string, value any, remove bool) error {
	ptr := ""
	for i, k := range path {
		ptr += "/" + escapePointer(k)
		if i < len(path)-1 {
			v, exists := j.get(path[:i+1])
			if !exists {
				if e := j.patch("add", ptr, map[string]any{}); e != nil {
					return e
				}
			} else if _, ok := v.(map[string]any); !ok {
				return errors.New("invalid_config: expected object")
			}
		}
	}
	_, exists := j.get(path)
	if remove && !exists {
		return nil
	}
	op := "add"
	if remove {
		op = "remove"
	}
	return j.patch(op, ptr, value)
}
func (j *jsonDoc) patch(op, path string, value any) error {
	b, e := json.Marshal([]map[string]any{{"op": op, "path": path, "value": value}})
	if e != nil {
		return e
	}
	if j.value.Patch(b) != nil {
		return errors.New("invalid_config: JSON field edit failed")
	}
	return nil
}
func (j *jsonDoc) bytes() []byte { j.value.Format(); return j.value.Pack() }
