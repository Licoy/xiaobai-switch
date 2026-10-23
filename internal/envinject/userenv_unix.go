//go:build !windows

package envinject

import "errors"

func ReadUser(string) (string, bool, error) {
	return "", false, errors.New("user environment requires Windows")
}
func WriteUser(string, string, bool) error { return errors.New("user environment requires Windows") }
