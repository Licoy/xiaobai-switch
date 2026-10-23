//go:build !darwin && !windows

package netx

import "context"

func platformProxy(context.Context) (string, error) { return "", nil }
