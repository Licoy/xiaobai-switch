package main

import (
	"encoding/json"
	"flag"
	"github.com/egoist/mygo"
	"github.com/licoy/xiaobai-switch/internal/buildinfo"
	"log"
	"os"
	"runtime"

	"github.com/licoy/xiaobai-switch/internal/app"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--native-smoke" {
			if err := json.NewEncoder(os.Stdout).Encode(struct {
				Version string `json:"version"`
				OS      string `json:"os"`
				Arch    string `json:"arch"`
				Native  bool   `json:"native"`
			}{buildinfo.Version, runtime.GOOS, runtime.GOARCH, true}); err != nil {
				log.Fatal(err)
			}
			return
		}
	}
	// The CLI runs the binary to generate bindings; this must never initialize storage or desktop integrations
	if os.Getenv("MYGO_GENERATE") != "" {
		if err := mygo.App.Run(); err != nil {
			log.Fatal(err)
		}
		return
	}
	useRealHome := flag.Bool("use-real-home", false, "use existing user data in development (the default)")
	devHome := flag.String("dev-home", "", "opt into an isolated development home")
	background := flag.Bool("background", false, "start in the system tray")
	flag.Parse()
	config, err := app.LoadConfigWithOptions(app.LaunchOptions{DevHome: *devHome, UseRealHome: *useRealHome})
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Run(config, *background); err != nil {
		log.Fatal(err)
	}
}
