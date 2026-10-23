package clidetect

import (
	"cmp"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type ProbeEnv struct {
	PathDirs  []string
	ExtraDirs []string
}

func FromProcess() (ProbeEnv, error) {
	home, e := paths.HomeDir()
	if e != nil {
		return ProbeEnv{}, e
	}
	extra, e := ExtraBinDirs(home)
	if e != nil {
		return ProbeEnv{}, e
	}
	return ProbeEnv{PathDirs: filepath.SplitList(os.Getenv("PATH")), ExtraDirs: extra}, nil
}
func ExtraBinDirs(home string) ([]string, error) {
	out := []string{}
	push := func(p string) {
		for _, v := range out {
			if v == p {
				return
			}
		}
		if p != "" {
			out = append(out, p)
		}
	}
	for _, p := range []string{".local/bin", ".npm-global/bin", ".volta/bin", ".bun/bin", ".cargo/bin", ".asdf/shims", ".local/share/mise/shims", ".mise/shims", ".fnm/aliases/default/bin", ".local/share/fnm/aliases/default/bin", "n/bin"} {
		push(filepath.Join(home, filepath.FromSlash(p)))
	}
	versions, e := versionDirs(filepath.Join(home, ".nvm", "versions", "node"))
	if e != nil {
		return nil, e
	}
	alias, e := optionalText(filepath.Join(home, ".nvm", "alias", "default"))
	if e != nil {
		return nil, e
	}
	alias = firstLine(alias)
	if strings.HasPrefix(alias, "lts/") {
		resolved, e := optionalText(filepath.Join(home, ".nvm", "alias", "lts", strings.TrimPrefix(alias, "lts/")))
		if e != nil {
			return nil, e
		}
		if value := firstLine(resolved); value != "" {
			alias = value
		}
	}
	for i, p := range versions {
		version := strings.TrimPrefix(filepath.Base(p), "v")
		a := strings.TrimPrefix(alias, "v")
		if a != "" && (version == a || strings.HasPrefix(version, a+".")) {
			preferred := p
			versions = append(versions[:i], versions[i+1:]...)
			versions = append([]string{preferred}, versions...)
			break
		}
	}
	for _, p := range versions {
		push(filepath.Join(p, "bin"))
	}
	for _, root := range []string{".local/share/fnm/node-versions", ".fnm/node-versions"} {
		versions, e = versionDirs(filepath.Join(home, filepath.FromSlash(root)))
		if e != nil {
			return nil, e
		}
		for _, p := range versions {
			push(filepath.Join(p, "installation", "bin"))
		}
	}
	switch runtime.GOOS {
	case "darwin":
		for _, p := range []string{"/opt/homebrew/bin", "/opt/homebrew/sbin", "/usr/local/bin", "/usr/local/sbin"} {
			push(p)
		}
	case "linux":
		push("/usr/local/bin")
		push("/home/linuxbrew/.linuxbrew/bin")
	case "windows":
		if p := os.Getenv("APPDATA"); p != "" {
			push(filepath.Join(p, "npm"))
		}
		if p := os.Getenv("LOCALAPPDATA"); p != "" {
			push(filepath.Join(p, "fnm"))
		}
		for _, p := range []string{filepath.Join(home, "AppData", "Roaming", "npm"), filepath.Join(home, "scoop", "shims"), `C:\Program Files\nodejs`, `C:\ProgramData\chocolatey\bin`} {
			push(p)
		}
	}
	return out, nil
}
func optionalText(path string) (string, error) {
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return "", nil
	}
	return string(b), e
}
func versionDirs(root string) ([]string, error) {
	entries, e := os.ReadDir(root)
	if errors.Is(e, os.ErrNotExist) {
		return []string{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, filepath.Join(root, entry.Name()))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a := strings.Split(strings.TrimPrefix(filepath.Base(out[i]), "v"), ".")
		b := strings.Split(strings.TrimPrefix(filepath.Base(out[j]), "v"), ".")
		for k := 0; k < min(len(a), len(b)); k++ {
			// Node version components sort numerically without integer overflow
			left, right := strings.TrimLeft(a[k], "0"), strings.TrimLeft(b[k], "0")
			if order := cmp.Compare(len(left), len(right)); order != 0 {
				return order > 0
			}
			if order := strings.Compare(left, right); order != 0 {
				return order > 0
			}
		}
		return len(a) > len(b)
	})
	return out, nil
}
func FindBinary(name string, env ProbeEnv) (string, error) {
	return findBinary(name, env, runtime.GOOS)
}
func findBinary(name string, env ProbeEnv, platform string) (string, error) {
	names := []string{name}
	if platform == "windows" {
		names = []string{name + ".exe", name + ".cmd", name + ".bat", name}
	}
	dirs := append(append([]string{}, env.PathDirs...), env.ExtraDirs...)
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		for _, file := range names {
			p := filepath.Join(dir, file)
			info, e := os.Stat(p)
			if errors.Is(e, os.ErrNotExist) {
				continue
			}
			if e != nil {
				return "", e
			}
			if info.Mode().IsRegular() && (platform == "windows" || info.Mode().Perm()&0111 != 0) {
				return filepath.Abs(p)
			}
		}
	}
	return "", nil
}
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
