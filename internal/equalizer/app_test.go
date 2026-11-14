package equalizer

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAppPublishesAllPresetCurves(t *testing.T) {
	var app App
	views := app.Presets()
	if len(views) != 11 {
		t.Fatalf("got %d presets, want 11", len(views))
	}
	for _, view := range views {
		if len(view.Curve) != 280 || len(view.Target) != 280 {
			t.Errorf("%s has %d response and %d target points, want 280 of each", view.ID, len(view.Curve), len(view.Target))
		}
	}
	if _, err := json.Marshal(views); err != nil {
		t.Fatalf("cannot encode preset curves for Wails: %v", err)
	}
}

func TestAppRejectsAmbiguousCable(t *testing.T) {
	app := newApp("", func() ([]openCable, error) {
		open := func() (io.WriteCloser, error) {
			t.Fatal("attempted to open a cable with multiple cables connected")
			return nil, nil
		}
		return []openCable{open, open}, nil
	})
	if status := app.CableStatus(); status.State != "multiple" {
		t.Errorf("cable state = %q, want multiple", status.State)
	}
	if _, err := app.ApplyPreset(t.Context(), "pref6"); err == nil {
		t.Error("apply succeeded with multiple cables connected")
	}
}

func TestAppAppliesRequestedPreset(t *testing.T) {
	var reports reportRecorder
	cable := &testCable{Writer: &reports}
	path := filepath.Join(t.TempDir(), "last-applied-preset")
	app := newApp(path, func() ([]openCable, error) {
		return []openCable{func() (io.WriteCloser, error) { return cable, nil }}, nil
	})
	if status := app.CableStatus(); status.State != "connected" {
		t.Errorf("cable state = %q, want connected", status.State)
	}
	result, err := app.ApplyPreset(t.Context(), "harman")
	if err != nil || result.Name != "Harman IE 2019" || !result.HistorySaved {
		t.Fatalf("apply result = %#v, error = %v", result, err)
	}
	if !cable.closed {
		t.Error("cable handle was left open")
	}
	const dataSize = 9 * 32
	if len(reports.sizes) != 10 || reports.Len() != dataSize+6 {
		t.Fatalf("got %d writes totaling %d bytes, want 10 writes totaling %d bytes", len(reports.sizes), reports.Len(), dataSize+6)
	}
	for i, size := range reports.sizes[:9] {
		if size != 32 {
			t.Errorf("report %d has %d bytes, want 32", i, size)
		}
	}
	// This digest comes from Kuba's shipped Harman data reports.
	data := reports.Bytes()
	const wantDigest = "128e80e1007713a343577649f4565f8900e83851701612d060d17d555e71526c"
	if got := fmt.Sprintf("%x", sha256.Sum256(data[:dataSize])); got != wantDigest {
		t.Errorf("Harman data reports SHA-256 = %s, want %s", got, wantDigest)
	}
	wantApply := []byte{0xcb, 0x01, 0x65, 0, 0x01, 0xff}
	if !bytes.Equal(data[dataSize:], wantApply) {
		t.Errorf("apply report = % x, want % x", data[dataSize:], wantApply)
	}
}

func TestLastAppliedPresetSurvivesRestartAndFailedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "last-applied-preset")
	writer := io.Discard
	app := newApp(path, func() ([]openCable, error) {
		return []openCable{func() (io.WriteCloser, error) { return &testCable{Writer: writer}, nil }}, nil
	})
	if got := app.LastAppliedPreset(); got != "" {
		t.Fatalf("first launch shows %q, want no history", got)
	}
	for _, id := range []string{"pref6", "harman"} {
		if result, err := app.ApplyPreset(t.Context(), id); err != nil || !result.HistorySaved {
			t.Fatalf("apply %s: result = %#v, error = %v", id, result, err)
		}
	}
	writer = new(shortWriter)
	if _, err := app.ApplyPreset(t.Context(), "pref6"); err == nil {
		t.Fatal("apply succeeded despite a short report write")
	}
	for _, instance := range []*App{app, newApp(path, nil)} {
		if got := instance.LastAppliedPreset(); got != "Harman IE 2019" {
			t.Errorf("history = %q, want the last successful apply", got)
		}
	}
}

func TestHistorySaveFailureDoesNotHideSuccessfulApply(t *testing.T) {
	path := t.TempDir() // A populated directory cannot be replaced with a history file.
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := newApp(path, func() ([]openCable, error) {
		return []openCable{func() (io.WriteCloser, error) {
			return &testCable{Writer: io.Discard}, nil
		}}, nil
	})
	result, err := app.ApplyPreset(t.Context(), "harman")
	if err != nil || result.HistorySaved || result.Name != "Harman IE 2019" {
		t.Fatalf("successful apply with unsaved history = %#v, error = %v", result, err)
	}
	if got := app.LastAppliedPreset(); got != result.Name {
		t.Errorf("in-memory history = %q, want %q", got, result.Name)
	}
}

func TestAppRejectsConcurrentApply(t *testing.T) {
	app := App{}
	app.write.Lock()
	defer app.write.Unlock()
	if _, err := app.ApplyPreset(t.Context(), "pref6"); !errors.Is(err, errApplyInProgress) {
		t.Errorf("apply error = %v, want write-in-progress error", err)
	}
}

type testCable struct {
	io.Writer
	closed bool
}

func (c *testCable) Close() error {
	c.closed = true
	return nil
}
