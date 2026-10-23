// The MyGo CLI owns -ldflags, so append our version to its actual compiler invocation.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

func main() {
	real, version := os.Getenv("NATIVE_REAL_GO"), os.Getenv("RELEASE_VERSION")
	if real == "" || !regexp.MustCompile(`^[1-9][0-9]*\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`).MatchString(version) {
		fmt.Fprintln(os.Stderr, "native Go wrapper requires real Go and RELEASE_VERSION")
		os.Exit(2)
	}
	args := append([]string(nil), os.Args[1:]...)
	if len(args) > 0 && args[0] == "build" {
		extra := " -X github.com/licoy/xiaobai-switch/internal/buildinfo.Version=" + version
		found := false
		for i := range args {
			if args[i] == "-ldflags" && i+1 < len(args) {
				args[i+1] += extra
				found = true
				break
			}
		}
		if !found {
			args = append([]string{"build", "-ldflags", extra}, args[1:]...)
		}
	}
	cmd := exec.Command(real, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			os.Exit(e.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
