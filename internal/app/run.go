package app

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/licoy/xiaobai-switch/internal/appbackup"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/store"
	"path/filepath"
	"strings"
	"time"
)

type host struct {
	positionTimer    *time.Timer
	menu             *mygo.Menu
	shell            *Shell
	main, panel      *mygo.Window
	tray             *mygo.Tray
	quitting, pinned bool
	trayTarget       int
	startupErr       error
}

func Run(config Config, background bool) (err error) {
	var shell *Shell
	defer func() {
		if err == nil && shell != nil && shell.RestartRequested {
			err = launchDevelopment(config)
		}
	}()
	app := mygo.App
	name := "XiaoBaiSwitch"
	if config.Development {
		name = fmt.Sprintf("XiaoBaiSwitch Dev %x", sha256.Sum256([]byte(config.Home)))[:33]
	}
	app.SetName(name)
	app.SetVersion(buildinfo.Version)
	app.SetPath(mygo.PathHome, config.Home)
	app.SetPath(mygo.PathUserData, config.DataDir)
	app.SetPath(mygo.PathLogs, filepath.Join(config.DataDir, "logs"))
	app.SetPath(mygo.PathCache, filepath.Join(config.DataDir, "cache"))
	if !app.RequestSingleInstanceLock() {
		return nil
	}
	if !config.Development || config.UseRealHome {
		running, checkErr := activateLegacy()
		if checkErr != nil {
			return checkErr
		}
		if running {
			return errors.New(config.Messages.Text("legacy_running"))
		}
	}
	var h *host
	instance, err := fileops.Acquire(filepath.Join(config.DataDir, "locks"), []string{"native_instance"}, nil)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, instance.Close()) }()
	if !config.Development || config.UseRealHome {
		release, legacyErr := reserveLegacy(func(args []string) {
			mygo.RunOnMain(func() {
				if h != nil {
					h.showMain()
					h.openArguments(args)
				}
			})
		})
		if legacyErr != nil {
			return legacyErr
		}
		defer func() { err = errors.Join(err, release()) }()
	}
	if _, err = appbackup.ApplyPending(config.DataDir); err != nil {
		return err
	}
	db, err := store.Open(config.DataDir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	shell, err = NewShell(config, db)
	if err != nil {
		return err
	}
	shell.FetchIcon = fetchSiteIcon
	defer func() { err = errors.Join(err, shell.closeManagers()) }()
	restore, restoreErr := appbackup.TakeRestoreResult(config.DataDir)
	if restoreErr != nil {
		return restoreErr
	}
	if restore != nil {
		if restore.Status == "applied" {
			shell.StartupNotice = shell.text("restore_completed")
			shell.WarningDetail = shell.safeErrorDetail(restore.Message)
		} else {
			shell.StartupNotice = shell.text("restore_failed")
			shell.WarningDetail = shell.safeErrorDetail(restore.Message)
		}
	}
	h = &host{shell: shell}
	app.OnBeforeQuit(func(event *mygo.QuitEvent) {
		if h.quitting {
			return
		}
		event.PreventDefault()
		h.quitting = true
		shell.Closing = true
		if shell.Confirm != nil && shell.Confirm.Dismiss != nil {
			shell.Confirm.Dismiss()
		}
		shell.Cancel()
		if h.positionTimer != nil {
			h.positionTimer.Stop()
		}
		shell.clearEditor()
		shell.clearKeyEditor()
		shell.BackupUI.Password = ""
		shell.Confirm = nil
		go func() {
			closeErr := shell.closeManagers()
			mygo.RunOnMain(func() { shell.ShutdownError = errors.Join(shell.ShutdownError, closeErr); app.Quit() })
		}()
	})
	app.OnActivate(func(bool) { h.showMain() })
	app.OnSecondInstance(func(args []string, _ string) { h.showMain(); h.openArguments(args) })
	app.OnOpenURL(func(raw string) { h.showMain(); h.importURL(raw) })
	app.OnWindowAllClosed(func() {
		if h.tray == nil {
			app.Quit()
		}
	})
	app.WhenReady(func() { h.ready(background) })
	app.OnWillQuit(func(*mygo.QuitEvent) {
		if h.tray != nil {
			h.tray.Destroy()
		}
	})
	if err = app.Run(); err != nil {
		return err
	}
	return errors.Join(h.startupErr, shell.ShutdownError)
}
func (h *host) showMain() {
	if h.main != nil {
		h.main.Show()
		h.main.Focus()
	}
}
func (h *host) changed(settings domain.AppSettings) error {
	if err := h.updateTrayMenu(settings); err != nil {
		return err
	}
	if !h.shell.Config.Development && settings.AutoStart != mygo.App.OpenAtLogin() {
		if err := mygo.App.SetOpenAtLogin(settings.AutoStart); err != nil {
			return err
		}
	}
	mygo.Theme.SetSource(mygo.ThemeSource(settings.ThemeMode))
	if h.main != nil {
		h.main.SetAlwaysOnTop(settings.AlwaysOnTop)
		h.main.Invalidate()
	}
	if h.panel != nil {
		h.panel.Invalidate()
	}
	return nil
}
func (h *host) openArguments(args []string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "xiaobaiswitch://") {
			h.importURL(arg)
		}
	}
}
