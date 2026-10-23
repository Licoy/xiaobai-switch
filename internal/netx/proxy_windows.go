//go:build windows

package netx

import (
	"context"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"golang.org/x/sys/windows/registry"
)

func platformProxy(ctx context.Context) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	key, e := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if errors.Is(e, registry.ErrNotExist) {
		return "", nil
	}
	if e != nil {
		return "", domain.NewError("network", "cannot open Windows proxy settings")
	}
	defer key.Close()
	enabled, _, e := key.GetIntegerValue("ProxyEnable")
	if errors.Is(e, registry.ErrNotExist) {
		return "", nil
	}
	if e != nil {
		return "", domain.NewError("network", "cannot read Windows proxy enable setting")
	}
	if enabled == 0 {
		return "", nil
	}
	server, _, e := key.GetStringValue("ProxyServer")
	if e != nil {
		return "", domain.NewError("network", "cannot read enabled Windows proxy server")
	}
	proxy := ParseWindowsProxyServer(server)
	if proxy == "" {
		return "", domain.NewError("invalid_config", "enabled Windows proxy has no server")
	}
	return proxy, nil
}
