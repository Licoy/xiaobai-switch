//go:build linux

package app

// Linux has no released legacy Tauri build; native instances use mygo plus the data-root file lock
func activateLegacy() (bool, error) { return false, nil }
func reserveLegacy(activate func([]string)) (func() error, error) {
	return func() error { return nil }, nil
}
