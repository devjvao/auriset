package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/sys/windows"
)

var setForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("SetForegroundWindow")

func installQuickPresetPicker(app *application.App, confirmation *PresetConfirmation) (open, toggle func()) {
	picker := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:           "quick-presets",
		Title:          "Presets rápidos",
		URL:            "/quick-presets.html",
		Width:          450,
		Height:         500,
		Hidden:         true,
		Frameless:      true,
		DisableResize:  true,
		AlwaysOnTop:    true,
		BackgroundType: application.BackgroundTypeTransparent,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
	})
	// These callbacks run on the UI thread.
	var previousForeground windows.HWND
	dismissPicker := func() {
		previous := previousForeground
		previousForeground = 0
		if previous != 0 && windows.IsWindowVisible(previous) && picker.IsFocused() {
			// Try while the picker still owns the foreground. Windows can deny the request.
			_, _, _ = setForegroundWindow.Call(uintptr(previous))
		}
		picker.Hide()
	}

	confirmation.install(app, picker, dismissPicker)
	picker.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if app.Context().Err() != nil {
			return
		}
		event.Cancel()
		application.InvokeSync(dismissPicker)
	})

	showPicker := func(toggle bool) {
		application.InvokeSync(func() {
			if app.Context().Err() != nil {
				return
			}
			if picker.IsVisible() {
				if toggle {
					dismissPicker()
				} else {
					picker.Focus()
				}
				return
			}
			foreground := windows.GetForegroundWindow()
			if foreground != windows.HWND(uintptr(picker.NativeWindow())) {
				previousForeground = foreground
			}
			confirmation.clear()
			picker.Center()
			picker.Show()
			picker.Focus()
			picker.EmitEvent("quick-presets:opened")
		})
	}
	return func() { showPicker(false) }, func() { showPicker(true) }
}
