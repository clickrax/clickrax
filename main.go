package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/service"
	"pbs-win-backup/internal/singleinstance"
	"pbs-win-backup/internal/version"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func writeServiceResult(path string, res models.ServiceActionResult) {
	if path == "" {
		return
	}
	data, _ := json.Marshal(res)
	_ = os.WriteFile(path, data, 0o600)
}

func runServiceCLI(action, resultPath string) {
	b := i18nconfig.FromConfig()
	var err error
	msg := b.T("main.done")
	switch action {
	case "install":
		err = service.Install()
		msg = b.T("service.action.installed_started")
	case "uninstall":
		err = service.Uninstall()
		msg = b.T("service.action.uninstalled")
	case "start":
		err = service.Start()
		msg = b.T("service.action.started")
	case "stop":
		err = service.Stop()
		msg = b.T("service.action.stopped")
	case "restart":
		err = service.Restart()
		msg = b.T("service.action.restarted")
	default:
		err = b.E("service.action.unknown")
	}
	res := models.ServiceActionResult{OK: err == nil, Message: msg}
	if err != nil {
		res.Message = err.Error()
	}
	writeServiceResult(resultPath, res)
	if err != nil {
		os.Exit(1)
	}
}

func main() {
	serviceFlag := false
	debugFlag := false
	for _, arg := range os.Args[1:] {
		if arg == "--service" {
			serviceFlag = true
		}
		if arg == "--service-debug" {
			serviceFlag = true
			debugFlag = true
		}
	}

	if serviceFlag {
		if err := service.RunService(debugFlag); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	installSvc := flag.Bool("install-service", false, "install Windows service")
	uninstallSvc := flag.Bool("uninstall-service", false, "uninstall Windows service")
	startSvc := flag.Bool("start-service", false, "start Windows service")
	stopSvc := flag.Bool("stop-service", false, "stop Windows service")
	restartSvc := flag.Bool("restart-service", false, "restart Windows service")
	resultFile := flag.String("result-file", "", "write service action result JSON")
	flag.Parse()

	action := ""
	switch {
	case *installSvc:
		action = "install"
	case *uninstallSvc:
		action = "uninstall"
	case *startSvc:
		action = "start"
	case *stopSvc:
		action = "stop"
	case *restartSvc:
		action = "restart"
	}
	if action != "" {
		runServiceCLI(action, *resultFile)
		return
	}

	if err := singleinstance.Acquire(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer singleinstance.Release()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     branding.Title + " v" + version.Version,
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Access-Control-Allow-Origin", "*")
					next.ServeHTTP(w, r)
				})
			},
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
