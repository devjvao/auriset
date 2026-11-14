package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

type TrayNotice struct {
	path string
	show func()
	hide func()

	mu           sync.Mutex
	ready        bool
	pending      bool
	acknowledged bool
}

func newTrayNotice(path string) *TrayNotice {
	return &TrayNotice{path: path}
}

// Ready is called after the webview has registered its tray notice listener.
func (n *TrayNotice) Ready() {
	n.mu.Lock()
	show := n.pending && !n.ready
	n.ready = true
	n.mu.Unlock()
	if show {
		n.show()
	}
}

func (n *TrayNotice) onClose() {
	n.mu.Lock()
	if n.pending {
		n.mu.Unlock()
		return
	}
	if n.acknowledged || n.hasAcknowledgment() {
		n.acknowledged = true
		n.mu.Unlock()
		n.hide()
		return
	}
	show := n.ready
	n.pending = true
	n.mu.Unlock()
	if show {
		n.show()
	}
}

func (n *TrayNotice) hasAcknowledgment() bool {
	if n.path == "" {
		return false
	}
	info, err := os.Stat(n.path)
	return err == nil && info.Mode().IsRegular()
}

// Dismiss remembers that the user acknowledged the notice and hides the window.
func (n *TrayNotice) Dismiss() {
	n.mu.Lock()
	if !n.pending {
		n.mu.Unlock()
		return
	}
	if err := n.saveAcknowledgment(); err != nil {
		// A settings error must not leave the user stuck at the notice.
		slog.Warn("could not save tray notice acknowledgment", "error", err)
	}
	n.pending = false
	n.acknowledged = true
	n.mu.Unlock()
	n.hide()
}

func (n *TrayNotice) saveAcknowledgment() error {
	if n.path == "" {
		return errors.New("user configuration directory is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(n.path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(n.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) && n.hasAcknowledgment() {
		return nil
	}
	if err != nil {
		return err
	}
	return file.Close()
}
