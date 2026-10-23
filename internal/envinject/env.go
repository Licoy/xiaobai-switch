package envinject

import (
	"errors"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

func ValidKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}
func Quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func Upsert(content []byte, key, value string, remove bool) ([]byte, error) {
	if !ValidKey(key) || strings.ContainsAny(value, "\x00\r\n") {
		return nil, errors.New("validation_failed: invalid environment key or value")
	}
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	var out []string
	found := false
	for _, line := range lines {
		trim := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		lhs, _, ok := strings.Cut(trim, "=")
		if ok && strings.TrimSpace(lhs) == key {
			if !remove && !found {
				out = append(out, "export "+key+"="+Quote(value))
				found = true
			}
			continue
		}
		out = append(out, line)
	}
	if !remove && !found {
		out = append(out, "export "+key+"="+Quote(value))
	}
	return []byte(strings.TrimLeft(strings.Join(out, "\n"), "\n") + "\n"), nil
}

const begin = "# >>> XiaoBaiSwitch >>>"
const end = "# <<< XiaoBaiSwitch <<<"

func SourceBlock(content []byte, path string) ([]byte, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return nil, errors.New("validation_failed: absolute env file required")
	}
	text := string(content)
	block := begin + "\n# Managed by XiaoBaiSwitch\n[ -f " + Quote(path) + " ] && . " + Quote(path) + "\n" + end + "\n"
	for _, markers := range [][2]string{{begin, end}, {"# >>> Xiaobai Switch >>>", "# <<< Xiaobai Switch <<<"}} {
		a := strings.Index(text, markers[0])
		b := strings.Index(text, markers[1])
		if a >= 0 || b >= 0 {
			if a < 0 || b < a {
				return nil, errors.New("invalid_config: unterminated managed shell block")
			}
			return []byte(text[:a] + block + strings.TrimPrefix(text[b+len(markers[1]):], "\n")), nil
		}
	}
	return []byte(strings.TrimRight(text, "\n") + "\n\n" + block), nil
}
func EffectiveMode(mode, platform string) (string, error) {
	switch mode {
	case "auto":
		if platform == "windows" {
			return "user_env", nil
		}
		return "shell_rc", nil
	case "shell_rc", "file_only":
		return mode, nil
	case "user_env":
		if platform != "windows" {
			return "", errors.New("validation_failed: user environment requires Windows")
		}
		return mode, nil
	default:
		return "", errors.New("validation_failed: unknown environment mode")
	}
}

type Request struct{ Mode, Platform, Home, EnvPath, Key, Value string }
type Plan struct {
	Mode                    string
	Files                   []fileops.Change
	UserKey                 string
	UserValue               string `json:"-"`
	UserRemove              bool
	UserExpectedFingerprint string `json:"-"`
}

func Build(r Request) (Plan, error) {
	p := Plan{}
	if r.Platform == "" {
		r.Platform = runtime.GOOS
	}
	mode, e := EffectiveMode(r.Mode, r.Platform)
	if e != nil {
		return p, e
	}
	p.Mode = mode
	b, exists, e := fileops.Read(r.EnvPath)
	if e != nil {
		return p, e
	}
	after, e := Upsert(b, r.Key, r.Value, false)
	if e != nil {
		return p, e
	}
	p.Files = append(p.Files, fileops.Change{Path: r.EnvPath, Before: b, Existed: exists, After: after})
	if mode == "user_env" {
		p.UserKey = r.Key
		p.UserValue = r.Value
		return p, nil
	}
	if mode != "shell_rc" {
		return p, nil
	}
	if !filepath.IsAbs(r.Home) {
		return p, fmt.Errorf("validation_failed: absolute shell home required")
	}
	for _, name := range []string{".zshrc", ".bashrc", ".profile"} {
		path := filepath.Join(r.Home, name)
		b, exists, e := fileops.Read(path)
		if e != nil {
			return p, e
		}
		if !exists && name != ".zshrc" {
			continue
		}
		after, e := SourceBlock(b, r.EnvPath)
		if e != nil {
			return p, e
		}
		p.Files = append(p.Files, fileops.Change{Path: path, Before: b, Existed: exists, After: after})
	}
	return p, nil
}
