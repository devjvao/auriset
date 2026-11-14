package main

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

//go:generate go -C ../../tools/winicon run .

const (
	windowIconResourceID = 3
	imageIcon            = 1
)

var loadWindowIcon = windows.NewLazySystemDLL("user32.dll").NewProc("LoadImageW")

func installWindowIcon(window *application.WebviewWindow) {
	// Keep the icon alive for the lifetime of the main window.
	loadSmallIcon := sync.OnceValue(func() w32.HICON {
		width := w32.GetSystemMetrics(w32.SM_CXSMICON)
		height := w32.GetSystemMetrics(w32.SM_CYSMICON)
		icon, _, _ := loadWindowIcon.Call(
			uintptr(w32.GetModuleHandle("")), windowIconResourceID, imageIcon,
			uintptr(width), uintptr(height), 0,
		)
		return w32.HICON(icon)
	})
	// Wails sets ICON_BIG from resource 3. Supply the small title-bar icon too.
	window.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		application.InvokeAsync(func() {
			hwnd := w32.HWND(uintptr(window.NativeWindow()))
			if hwnd == 0 {
				return
			}
			if icon := loadSmallIcon(); icon != 0 {
				w32.SetWindowIcon(hwnd, icon)
			}
		})
	})
}
