package app

import (
	"github.com/egoist/mygo"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
)

func (s *Shell) openDirectory(path string) {
	validated, err := paths.ValidatePath(path)
	if err != nil {
		s.result(err)
		return
	}
	if _, err = os.Stat(validated); err != nil {
		s.result(err)
		return
	}
	if err = mygo.Shell.OpenPath(validated); err != nil {
		s.result(err)
	}
}
func (s *Shell) openURL(url string) {
	if err := mygo.Shell.OpenExternal(url); err != nil {
		s.result(err)
	}
}
