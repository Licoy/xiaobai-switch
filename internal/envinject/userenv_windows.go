//go:build windows

package envinject

import (
	"errors"
	"golang.org/x/sys/windows/registry"
)

func ReadUser(key string) (string, bool, error) {
	if !ValidKey(key) {
		return "", false, errors.New("invalid environment key")
	}
	k, e := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
	if errors.Is(e, registry.ErrNotExist) {
		return "", false, nil
	}
	if e != nil {
		return "", false, e
	}
	defer k.Close()
	s, _, e := k.GetStringValue(key)
	if errors.Is(e, registry.ErrNotExist) {
		return "", false, nil
	}
	return s, e == nil, e
}
func WriteUser(key, value string, remove bool) error {
	if !ValidKey(key) {
		return errors.New("invalid environment key")
	}
	k, _, e := registry.CreateKey(registry.CURRENT_USER, "Environment", registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if remove {
		e = k.DeleteValue(key)
		if errors.Is(e, registry.ErrNotExist) {
			return nil
		}
		return e
	}
	return k.SetStringValue(key, value)
}
