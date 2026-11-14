package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"auriset/internal/config"
)

const defaultPresetShortcut = "Ctrl+Alt+K"

type shortcutRegistrar interface {
	Register(string, func()) error
	Unregister(string) error
}

// PresetShortcut manages the saved and active global shortcut for the picker.
type PresetShortcut struct {
	path  string
	write func(string, []byte) error
	ready chan struct{}

	mu        sync.Mutex
	active    string
	registrar shortcutRegistrar
	open      func()
	logger    *slog.Logger
}

func newPresetShortcut(path string) *PresetShortcut {
	return &PresetShortcut{path: path, write: config.WriteFile, ready: make(chan struct{})}
}

func (s *PresetShortcut) bind(registrar shortcutRegistrar, open func(), logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer close(s.ready)

	s.registrar = registrar
	s.open = open
	s.logger = logger

	saved, err := loadPresetShortcut(s.path)
	if err != nil {
		logger.Warn("could not load preset shortcut", "error", err)
	}
	if saved != "" {
		if err := s.register(saved); err == nil {
			s.active = saved
			return
		} else {
			logger.Warn("saved preset shortcut is unavailable", "shortcut", saved, "error", err)
		}
		if saved == defaultPresetShortcut {
			return
		}
	}
	if err := s.register(defaultPresetShortcut); err != nil {
		logger.Warn("default preset shortcut is unavailable", "shortcut", defaultPresetShortcut, "error", err)
		return
	}
	s.active = defaultPresetShortcut
}

// Get returns the active shortcut, or an empty string if none could be registered.
func (s *PresetShortcut) Get(ctx context.Context) (string, error) {
	if err := s.waitReady(ctx); err != nil {
		return "", err
	}
	return s.current(), nil
}

func (s *PresetShortcut) current() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Set changes the shortcut only after Windows accepts it and its preference is saved.
func (s *PresetShortcut) Set(ctx context.Context, value string) (string, error) {
	shortcut, err := normalizePresetShortcut(value)
	if err != nil {
		return "", err
	}
	if err := s.waitReady(ctx); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := shortcut != s.active
	if changed {
		if err := s.register(shortcut); err != nil {
			s.logger.Warn("could not register preset shortcut", "shortcut", shortcut, "error", err)
			return "", errors.New("atalho indisponível. Escolha outra combinação")
		}
	}
	if err := s.write(s.path, []byte(shortcut+"\n")); err != nil {
		if changed {
			s.unregister(shortcut)
		}
		s.logger.Warn("could not save preset shortcut", "error", err)
		return "", errors.New("não foi possível salvar o atalho")
	}

	if changed {
		previous := s.active
		s.active = shortcut
		if previous != "" {
			s.unregister(previous)
		}
	}
	return shortcut, nil
}

func (s *PresetShortcut) waitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *PresetShortcut) register(shortcut string) error {
	return s.registrar.Register(wailsShortcut(shortcut), func() {
		if s.current() == shortcut {
			s.open()
		}
	})
}

func (s *PresetShortcut) unregister(shortcut string) {
	if err := s.registrar.Unregister(wailsShortcut(shortcut)); err != nil {
		s.logger.Warn("could not release preset shortcut", "shortcut", shortcut, "error", err)
	}
}

func loadPresetShortcut(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read shortcut configuration: %w", err)
	}
	return normalizePresetShortcut(string(raw))
}

// Wails accepts Super as input even though it displays the modifier as Win.
func wailsShortcut(shortcut string) string {
	return strings.ReplaceAll(shortcut, "Win+", "Super+")
}

func normalizePresetShortcut(value string) (string, error) {
	parts := strings.Split(value, "+")
	end := len(parts) - 1
	key := strings.ToUpper(strings.TrimSpace(parts[end]))
	if !isPresetShortcutKey(key) {
		return "", errors.New("escolha uma letra, um número ou uma tecla F, exceto F12")
	}
	modifiers := make(map[string]bool, end)
	for _, part := range parts[:end] {
		modifier := strings.ToLower(strings.TrimSpace(part))
		switch modifier {
		case "super":
			modifier = "win"
		case "ctrl", "alt", "shift", "win":
		default:
			return "", errors.New("use Ctrl, Alt, Shift ou Win como modificadores")
		}
		if modifiers[modifier] {
			return "", errors.New("não repita as teclas modificadoras")
		}
		modifiers[modifier] = true
	}
	if !modifiers["ctrl"] && !modifiers["alt"] && !modifiers["win"] {
		return "", errors.New("inclua Ctrl, Alt ou Win no atalho")
	}
	var canonical []string
	for _, modifier := range []string{"Ctrl", "Alt", "Shift", "Win"} {
		if modifiers[strings.ToLower(modifier)] {
			canonical = append(canonical, modifier)
		}
	}
	return strings.Join(append(canonical, key), "+"), nil
}

func isPresetShortcutKey(key string) bool {
	if len(key) == 1 {
		return key[0] >= 'A' && key[0] <= 'Z' || key[0] >= '0' && key[0] <= '9'
	}
	suffix, ok := strings.CutPrefix(key, "F")
	if !ok {
		return false
	}
	n, err := strconv.Atoi(suffix)
	return err == nil && n >= 1 && n <= 24 && n != 12 && strconv.Itoa(n) == suffix
}
