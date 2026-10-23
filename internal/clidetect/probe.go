package clidetect

import (
	"bytes"
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var versionToken = regexp.MustCompile(`[0-9]\.[0-9]`)

func looksLikeVersion(s string) bool {
	lower := strings.ToLower(s)
	for _, bad := range []string{"error", "enoent", "not found", "errno"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return versionToken.MatchString(s)
}

type limitedOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	size := len(p)
	remaining := 64*1024 - b.Len()
	if remaining > 0 {
		b.Buffer.Write(p[:min(remaining, size)])
	}
	if size > remaining {
		b.overflow = true
	}
	return size, nil
}
func ProbeWith(ctx context.Context, target domain.TargetKind, env ProbeEnv) (domain.CliToolInfo, error) {
	if !target.Valid() {
		return domain.CliToolInfo{}, domain.NewError("validation_failed", "unknown CLI target")
	}
	name := map[domain.TargetKind]string{domain.ClaudeCode: "claude", domain.Codex: "codex", domain.Pi: "pi", domain.Prime: "prime-agent"}[target]
	path, e := FindBinary(name, env)
	if e != nil {
		return domain.CliToolInfo{}, e
	}
	if path == "" && target == domain.Prime {
		path, e = FindBinary("prime", env)
		if e != nil {
			return domain.CliToolInfo{}, e
		}
	}
	info := domain.CliToolInfo{Kind: target, Installed: path != ""}
	if path == "" {
		return info, nil
	}
	info.Path = &path
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, "--version")
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".cmd" || ext == ".bat" {
			cmd = exec.CommandContext(probeCtx, "cmd", "/d", "/c", path, "--version")
		}
	}
	dirs := append([]string{filepath.Dir(path)}, env.PathDirs...)
	dirs = append(dirs, env.ExtraDirs...)
	cmd.Env = append(os.Environ(), "PATH="+strings.Join(dirs, string(os.PathListSeparator)))
	cmd.WaitDelay = 100 * time.Millisecond
	var stdout, stderr limitedOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	e = cmd.Run()
	if probeCtx.Err() != nil {
		return info, probeCtx.Err()
	}
	if stdout.overflow || stderr.overflow {
		return info, domain.NewError("invalid_response", "CLI version output exceeds size limit")
	}
	line := firstLine(stdout.String())
	if line == "" {
		line = firstLine(stderr.String())
	}
	if looksLikeVersion(line) {
		info.Version = &line
	}
	var exit *exec.ExitError
	if e != nil && !errors.As(e, &exit) {
		return info, e
	}
	return info, nil
}
func Detect(ctx context.Context, env ProbeEnv) ([]domain.CliToolInfo, error) {
	tools, _, err := detect(ctx, env)
	return tools, err
}
func detect(ctx context.Context, env ProbeEnv) ([]domain.CliToolInfo, map[domain.TargetKind]string, error) {
	targets := []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime}
	out := make([]domain.CliToolInfo, len(targets))
	errs := make([]error, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, target domain.TargetKind) { defer wg.Done(); out[i], errs[i] = ProbeWith(ctx, target, env) }(i, t)
	}
	wg.Wait()
	targetErrors := map[domain.TargetKind]string{}
	for i, err := range errs {
		if err != nil {
			targetErrors[targets[i]] = err.Error()
		}
	}
	return out, targetErrors, errors.Join(errs...)
}
