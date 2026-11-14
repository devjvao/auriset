package main

import (
	_ "embed"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const desktopTitle = "Auriset"

//go:embed tray.png
var trayIcon []byte

func installTray(app *application.App, window *application.WebviewWindow, notice *TrayNotice, openPicker func()) func() {
	var quitting atomic.Bool
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip(desktopTitle)

	showWindow := func() {
		application.InvokeSync(func() {
			if quitting.Load() || app.Context().Err() != nil {
				return
			}
			window.Show()
			if window.IsMinimised() {
				window.UnMinimise()
			}
			window.Focus()
		})
	}
	notice.show = func() {
		application.InvokeSync(func() {
			if !quitting.Load() {
				showWindow()
				window.EmitEvent("tray:notice")
			}
		})
	}
	notice.hide = func() {
		application.InvokeSync(func() {
			if !quitting.Load() {
				window.Hide()
			}
		})
	}
	tray.OnClick(showWindow)
	menu := app.NewMenu()
	menu.Add("Abrir " + desktopTitle).OnClick(func(*application.Context) { showWindow() })
	menu.Add("Presets rápidos").OnClick(func(*application.Context) { openPicker() })
	menu.Add("Sair").OnClick(func(*application.Context) {
		quitting.Store(true)
		app.Quit()
	})
	tray.SetMenu(menu)

	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if quitting.Load() {
			return
		}
		event.Cancel()
		notice.onClose()
	})
	return showWindow
}
