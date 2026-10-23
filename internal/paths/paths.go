package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AppPaths struct {
	AppDir        string `json:"appDir"`
	DBPath        string `json:"dbPath"`
	MasterKeyPath string `json:"masterKeyPath"`
	BackupsDir    string `json:"backupsDir"`
	AppBackupsDir string `json:"appBackupsDir"`
	CodexEnvPath  string `json:"codexEnvPath"`
	LogsDir       string `json:"logsDir"`
}

func HomeDir() (string, error) {
	if p := strings.TrimSpace(os.Getenv("XIAOBAI_DEV_HOME")); p != "" {
		return developmentHome(p)
	}
	return os.UserHomeDir()
}
func homeJoin(parts ...string) (string, error) {
	h, e := HomeDir()
	if e != nil {
		return "", e
	}
	return validateResolved(filepath.Join(append([]string{h}, parts...)...))
}
func AppDir() (string, error) { return homeJoin(".xiaobai-switch") }
func appJoin(parts ...string) (string, error) {
	return homeJoin(append([]string{".xiaobai-switch"}, parts...)...)
}
func DBPath() (string, error)        { return appJoin("xiaobai-switch.db") }
func MasterKeyPath() (string, error) { return appJoin("master.key") }
func BackupsDir() (string, error)    { return appJoin("backups") }
func AppBackupsDir() (string, error) { return appJoin("backups", "app") }
func LocksDir() (string, error)      { return appJoin("locks") }
func CodexEnvPath() (string, error)  { return appJoin("env", "codex.env") }
func EnsureAppDirs() (string, error) {
	root, e := AppDir()
	if e != nil {
		return "", e
	}
	for _, p := range []string{"", "backups", "backups/app", "env", "locks", "logs"} {
		if e = os.MkdirAll(filepath.Join(root, p), 0700); e != nil {
			return "", e
		}
	}
	return root, nil
}
func resolve(override, env string, parts ...string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return validateResolved(override)
	}
	if os.Getenv("XIAOBAI_DEV_HOME") == "" && env != "" {
		if p := os.Getenv(env); strings.TrimSpace(p) != "" {
			return validateResolved(p)
		}
	}
	return homeJoin(parts...)
}
func ResolveClaudeHome(p string) (string, error) { return resolve(p, "", ".claude") }
func ResolveCodexHome(p string) (string, error)  { return resolve(p, "CODEX_HOME", ".codex") }
func ResolvePiAgentDir(p string) (string, error) {
	return resolve(p, "PI_CODING_AGENT_DIR", ".pi", "agent")
}
func ResolvePrimeAgentDir(p string) (string, error) {
	return resolve(p, "PRIME_AGENT_CODING_AGENT_DIR", ".prime", "agent")
}

// ResolveExisting evaluates existing ancestors without creating directories
func ResolveExisting(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("validation_failed: path must be absolute")
	}
	current := filepath.Clean(path)
	tail := []string{}
	for {
		_, e := os.Lstat(current)
		if e == nil {
			real, e := filepath.EvalSymlinks(current)
			if e != nil {
				return "", e
			}
			for i := len(tail) - 1; i >= 0; i-- {
				real = filepath.Join(real, tail[i])
			}
			return real, nil
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", e
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}
func developmentHome(path string) (string, error) {
	root, e := ResolveExisting(path)
	if e != nil {
		return "", e
	}
	if filepath.Dir(root) == root {
		return "", fmt.Errorf("validation_failed: development home cannot be a filesystem root")
	}
	real, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	real, e = ResolveExisting(real)
	if e != nil {
		return "", e
	}
	if inside(root, real) {
		return "", fmt.Errorf("validation_failed: development home cannot be the system home")
	}
	for _, private := range []string{".claude", ".codex", ".pi", ".prime", ".xiaobai-switch"} {
		p := filepath.Join(real, private)
		if inside(p, root) {
			return "", fmt.Errorf("validation_failed: development home cannot use a live configuration directory")
		}
	}
	return root, nil
}
func inside(root, path string) bool {
	rel, e := filepath.Rel(root, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func validateResolved(path string) (string, error) {
	resolved, e := ResolveExisting(path)
	if e != nil {
		return "", e
	}
	dev := strings.TrimSpace(os.Getenv("XIAOBAI_DEV_HOME"))
	if dev == "" {
		return resolved, nil
	}
	root, e := developmentHome(dev)
	if e != nil {
		return "", e
	}
	if !inside(root, resolved) {
		return "", fmt.Errorf("validation_failed: path escapes development home")
	}
	return resolved, nil
}

// ValidatePath resolves symlinks and enforces development isolation for explicit destinations
func ValidatePath(path string) (string, error) { return validateResolved(path) }
