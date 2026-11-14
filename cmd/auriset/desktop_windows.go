package main

import (
	"os"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"auriset/frontend"
	"auriset/internal/config"
	"auriset/internal/equalizer"
)

func runDesktop() error {
	notice := newTrayNotice(config.Path("tray-notice-acknowledged"))
	confirmation := &PresetConfirmation{}
	shortcut := newPresetShortcut(config.Path("preset-shortcut"))
	// A second launch can arrive before Wails has created the main window.
	reopen := make(chan struct{}, 1)
	app := application.New(application.Options{
		Name: desktopTitle,
		Icon: trayIcon,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "audio.kuba.equalizer",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if isWindowsStartup(data.Args) {
					return
				}
				select {
				case reopen <- struct{}{}:
				default:
				}
			},
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontend.Assets),
		},
		Services: []application.Service{
			application.NewService(&AppInfo{}),
			application.NewService(equalizer.NewApp(config.Path("last-applied-preset"))),
			application.NewService(notice),
			application.NewService(confirmation),
			application.NewService(&WindowsStartup{}),
			application.NewService(shortcut),
		},
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            desktopTitle,
		URL:              "/",
		Width:            1100,
		Height:           740,
		MinWidth:         620,
		MinHeight:        560,
		Hidden:           isWindowsStartup(os.Args),
		BackgroundColour: application.NewRGB(18, 17, 15),
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
	})
	installWindowIcon(window)
	openPicker, togglePicker := installQuickPresetPicker(app, confirmation)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		shortcut.bind(app.GlobalShortcut, togglePicker, app.Logger)
	})
	showWindow := installTray(app, window, notice, openPicker)
	// ApplicationStarted can fire before the hidden main window can be shown.
	// Start consuming queued launches when that window's runtime is ready.
	reopenWhenReady := sync.OnceFunc(func() {
		go func() {
			for {
				select {
				case <-app.Context().Done():
					return
				case <-reopen:
					showWindow()
				}
			}
		}()
	})
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) { reopenWhenReady() })
	return app.Run()
}
