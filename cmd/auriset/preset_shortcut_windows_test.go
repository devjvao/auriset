package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePresetShortcut(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
	}{
		{name: "keeps default", input: "Ctrl+Alt+K", want: "Ctrl+Alt+K"},
		{name: "canonicalizes", input: " win + shift + alt + ctrl + f24 ", want: "Ctrl+Alt+Shift+Win+F24"},
		{name: "accepts Super", input: "Super+2", want: "Win+2"},
		{name: "rejects bare key", input: "K"},
		{name: "rejects shift-only", input: "Shift+K"},
		{name: "rejects invalid modifier", input: "Alt+Magic+K"},
		{name: "rejects repeated modifier", input: "Alt+Alt+K"},
		{name: "rejects debugger key", input: "Ctrl+F12"},
		{name: "rejects unsupported key", input: "Ctrl+F25"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePresetShortcut(tt.input)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("normalizePresetShortcut(%q) = %q; expected error", tt.input, got)
				}
			} else if err != nil || got != tt.want {
				t.Fatalf("normalizePresetShortcut(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
		})
	}
}

func TestPresetShortcutChangePreservesCurrentOnFailure(t *testing.T) {
	for _, tt := range []struct {
		name           string
		conflict       bool
		persistFailure bool
	}{
		{name: "occupied", conflict: true},
		{name: "cannot save", persistFailure: true},
		{name: "successful change"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "preset-shortcut")
			if err := os.WriteFile(path, []byte("Ctrl+Alt+K\n"), 0600); err != nil {
				t.Fatal(err)
			}
			s := newPresetShortcut(path)
			registrar := newFakeShortcutRegistrar()
			if tt.conflict {
				registrar.occupied["Ctrl+Shift+G"] = true
			}
			s.bind(registrar, func() {}, discardLogger())
			if tt.persistFailure {
				s.write = func(string, []byte) error {
					return errors.New("permission denied")
				}
			}
			got, err := s.Set(t.Context(), "Shift+Ctrl+g")
			want := "Ctrl+Shift+G"
			if tt.conflict || tt.persistFailure {
				want = "Ctrl+Alt+K"
				if err == nil {
					t.Fatal("Set succeeded despite failure")
				}
			} else if err != nil || got != want {
				t.Fatalf("Set returned %q, %v; want %q", got, err, want)
			}
			if got, err := s.Get(t.Context()); err != nil || got != want {
				t.Fatalf("Get = %q, %v; want %q", got, err, want)
			}
			if len(registrar.active) != 1 || registrar.active[want] == nil {
				t.Fatalf("registered shortcuts = %v; expected only %q", registrar.active, want)
			}
			saved, err := os.ReadFile(path)
			if err != nil || string(saved) != want+"\n" {
				t.Fatalf("saved shortcut = %q, %v; want %q", saved, err, want)
			}
		})
	}
}

func TestPresetShortcutStartupFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preset-shortcut")
	if err := os.WriteFile(path, []byte("Win+G\n"), 0600); err != nil {
		t.Fatal(err)
	}
	registrar := newFakeShortcutRegistrar()
	registrar.occupied["Super+G"] = true
	s := newPresetShortcut(path)
	s.bind(registrar, func() {}, discardLogger())
	if got, err := s.Get(t.Context()); err != nil || got != "Ctrl+Alt+K" {
		t.Errorf("Get = %q, %v; want default when saved shortcut is occupied", got, err)
	}
	saved, err := os.ReadFile(path)
	if err != nil || string(saved) != "Win+G\n" {
		t.Fatalf("saved shortcut = %q, %v; must retain preference for the next launch", saved, err)
	}
	if _, err := s.Set(t.Context(), "Ctrl+Alt+K"); err != nil {
		t.Fatal(err)
	}
	saved, err = os.ReadFile(path)
	if err != nil || string(saved) != "Ctrl+Alt+K\n" {
		t.Fatalf("reset setting = %q, %v; want persisted default", saved, err)
	}
}

func TestPresetShortcutCanCancelBeforeStartup(t *testing.T) {
	s := newPresetShortcut("")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.Get(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get error = %v; want cancellation", err)
	}
	if _, err := s.Set(ctx, "Ctrl+Alt+K"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Set error = %v; want cancellation", err)
	}
}

type fakeShortcutRegistrar struct {
	active   map[string]func()
	occupied map[string]bool
}

func newFakeShortcutRegistrar() *fakeShortcutRegistrar {
	return &fakeShortcutRegistrar{active: map[string]func(){}, occupied: map[string]bool{}}
}

func (f *fakeShortcutRegistrar) Register(key string, callback func()) error {
	if f.occupied[key] || f.active[key] != nil {
		return errors.New("shortcut occupied")
	}
	f.active[key] = callback
	return nil
}

func (f *fakeShortcutRegistrar) Unregister(key string) error {
	delete(f.active, key)
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
