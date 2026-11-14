package main

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	confirmationDuration = 2200 * time.Millisecond
	confirmationWidth    = 342
	confirmationHeight   = 96
	confirmationInset    = 14

	wsExTopmost             = 0x00000008
	wsExToolWindow          = 0x00000080
	wsExControlParent       = 0x00010000
	wsExNoRedirectionBitmap = 0x00200000
	wsExNoActivate          = 0x08000000
)

// PresetConfirmation coordinates the picker and its confirmation on the UI thread.
type PresetConfirmation struct {
	app           *application.App
	window        *application.WebviewWindow
	picker        *application.WebviewWindow
	dismissPicker func()

	ready      bool
	pending    string
	timer      *time.Timer
	generation uint64
}

func (c *PresetConfirmation) install(
	app *application.App,
	picker *application.WebviewWindow,
	dismissPicker func(),
) {
	c.app = app
	c.picker = picker
	c.dismissPicker = dismissPicker
	c.window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:           "preset-confirmation",
		Title:          "Preset aplicado",
		URL:            "/preset-confirmation.html",
		Width:          confirmationWidth,
		Height:         confirmationHeight,
		Hidden:         true,
		Frameless:      true,
		DisableResize:  true,
		AlwaysOnTop:    true,
		BackgroundType: application.BackgroundTypeTransparent,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
			// ExStyle replaces the defaults; keep Wails' transparent composition flags.
			ExStyle: wsExTopmost | wsExToolWindow | wsExControlParent |
				wsExNoRedirectionBitmap | wsExNoActivate,
		},
	})
	app.OnShutdown(c.clear)
}

// Ready is called after the confirmation page has subscribed to its event.
func (c *PresetConfirmation) Ready() {
	application.InvokeSync(func() {
		// Wails uses SW_SHOW, which requests activation. A disabled toast cannot take focus.
		c.window.SetEnabled(false)
		c.ready = true
		if c.pending != "" {
			c.show()
		}
	})
}

// ShowAfterPicker hides the picker before displaying the successful write.
func (c *PresetConfirmation) ShowAfterPicker(name string) {
	if name == "" {
		return
	}
	application.InvokeSync(func() {
		if c.app.Context().Err() != nil {
			return
		}
		if screen, err := c.picker.GetScreen(); err == nil && screen != nil {
			area := screen.WorkArea
			c.window.SetBounds(application.Rect{
				X:      area.X + max(0, area.Width-confirmationWidth-confirmationInset),
				Y:      area.Y + max(0, area.Height-confirmationHeight-confirmationInset),
				Width:  confirmationWidth,
				Height: confirmationHeight,
			})
		}
		c.dismissPicker()
		c.pending = name
		if c.ready {
			c.show()
		}
	})
}

func (c *PresetConfirmation) show() {
	c.window.EmitEvent("preset:confirmation", c.pending)
	c.pending = ""
	c.window.Show()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.generation++
	generation := c.generation
	c.timer = time.AfterFunc(confirmationDuration, func() {
		if c.app.Context().Err() != nil {
			return
		}
		application.InvokeAsync(func() {
			if c.app.Context().Err() == nil && c.generation == generation {
				c.clear()
			}
		})
	})
}

func (c *PresetConfirmation) clear() {
	c.pending = ""
	c.generation++
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.window.Hide()
}
