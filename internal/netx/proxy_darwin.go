//go:build darwin

package netx

import (
	"context"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os/exec"
	"strings"
)

func platformProxy(ctx context.Context) (string, error) {
	out, e := exec.CommandContext(ctx, "/usr/sbin/scutil", "--proxy").Output()
	if e != nil {
		return "", domain.NewError("network", "cannot read macOS system proxy settings")
	}
	proxy := ParseScutilProxy(string(out))
	if proxy == "" {
		for _, line := range strings.Split(string(out), "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
			key = strings.TrimSpace(key)
			if ok && (key == "HTTPEnable" || key == "HTTPSEnable" || key == "SOCKSEnable") && strings.TrimSpace(value) == "1" {
				return "", domain.NewError("invalid_config", "enabled macOS proxy has no valid endpoint")
			}
		}
	}
	return proxy, nil
}
