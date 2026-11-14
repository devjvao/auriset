package equalizer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"kuba-equalizer/internal/config"
)

type openCable func() (io.WriteCloser, error)

type App struct {
	find  func() ([]openCable, error)
	write sync.Mutex

	historyPath string
	historyMu   sync.RWMutex
	lastApplied string
}

func newApp(historyPath string, find func() ([]openCable, error)) *App {
	app := &App{find: find, historyPath: historyPath}
	if historyPath == "" {
		return app
	}
	data, err := os.ReadFile(historyPath)
	if err == nil {
		if p, ok := findPreset(strings.TrimSpace(string(data))); ok {
			app.lastApplied = p.name
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("could not read last applied preset", "error", err)
	}
	return app
}

// LastAppliedPreset reports the latest successful write by the app, not the cable's current state.
func (a *App) LastAppliedPreset() string {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()
	return a.lastApplied
}

type PresetView struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Group       string       `json:"group"`
	Description string       `json:"description"`
	PreampDB    float64      `json:"preampDb"`
	Curve       []CurvePoint `json:"curve"`
	Target      []CurvePoint `json:"target"`
	TargetName  string       `json:"targetName"`
}

type CableView struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

type ApplyResult struct {
	Name         string `json:"name"`
	HistorySaved bool   `json:"historySaved"`
}

var (
	errApplyInProgress = errors.New("já há um preset sendo enviado")
	errApplyCanceled   = errors.New("o envio foi cancelado")
)

func (a *App) Presets() []PresetView {
	views := make([]PresetView, 0, len(presets))
	for _, p := range presets {
		curve, target, targetName := websiteGraph(p)
		views = append(views, PresetView{
			ID: p.id, Name: p.name, Group: p.group, Description: p.description,
			PreampDB: p.preampDB, Curve: curve, Target: target, TargetName: targetName,
		})
	}
	return views
}

func (a *App) CableStatus() CableView {
	devices, err := a.find()
	return cableStatus(len(devices), err)
}

func cableStatus(count int, err error) CableView {
	switch {
	case err != nil:
		return CableView{State: "error", Message: "Não foi possível verificar a conexão do cabo."}
	case count == 0:
		return CableView{State: "disconnected", Message: "Conecte o cabo USB-C com DSP do Auri a este computador."}
	case count > 1:
		return CableView{State: "multiple", Message: "Deixe apenas um cabo Auri conectado para aplicar um preset."}
	default:
		return CableView{State: "connected", Message: "Cabo USB-C com DSP do Auri"}
	}
}

func (a *App) ApplyPreset(ctx context.Context, id string) (ApplyResult, error) {
	p, ok := findPreset(id)
	if !ok {
		return ApplyResult{}, errors.New("preset desconhecido")
	}
	if !a.write.TryLock() {
		return ApplyResult{}, errApplyInProgress
	}
	defer a.write.Unlock()
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, errApplyCanceled
	}

	devices, err := a.find()
	if status := cableStatus(len(devices), err); status.State != "connected" {
		return ApplyResult{}, errors.New(status.Message)
	}
	if err := applyToCable(ctx, devices[0], p); err != nil {
		if errors.Is(err, context.Canceled) {
			return ApplyResult{}, errApplyCanceled
		}
		slog.Warn("could not send preset to cable", "preset", p.id, "error", err)
		return ApplyResult{}, errors.New("verifique a conexão do cabo e tente novamente")
	}
	saveErr := config.WriteFile(a.historyPath, []byte(p.id+"\n"))
	if saveErr != nil {
		slog.Warn("could not save last applied preset", "preset", p.id, "error", saveErr)
	}
	a.historyMu.Lock()
	a.lastApplied = p.name
	a.historyMu.Unlock()
	return ApplyResult{Name: p.name, HistorySaved: saveErr == nil}, nil
}

func applyToCable(ctx context.Context, open openCable, p preset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	handle, err := open()
	if err != nil {
		return fmt.Errorf("open Auri HID collection: %w", err)
	}
	defer func() { _ = handle.Close() }()
	return applyPreset(ctx, handle, p)
}
