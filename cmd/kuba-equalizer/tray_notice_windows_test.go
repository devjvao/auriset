package main

import (
	"path/filepath"
	"testing"
)

func TestTrayNoticeAcknowledgmentSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kuba-equalizer", "tray-notice-acknowledged")
	var shown, hidden int
	show := func() { shown++ }
	hide := func() { hidden++ }
	notice := newTrayNotice(path)
	notice.show = show
	notice.hide = hide

	notice.onClose()
	if shown != 0 || hidden != 0 {
		t.Fatalf("close before frontend ready: shown=%d, hidden=%d", shown, hidden)
	}
	notice.Ready()
	notice.onClose()
	if shown != 1 || hidden != 0 {
		t.Fatalf("first close and duplicate: shown=%d, hidden=%d", shown, hidden)
	}
	notice.Dismiss()
	if hidden != 1 {
		t.Fatalf("acknowledgment hid the window %d times, want 1", hidden)
	}

	restarted := newTrayNotice(path)
	restarted.show = show
	restarted.hide = hide
	restarted.Ready()
	restarted.onClose()
	if shown != 1 || hidden != 2 {
		t.Fatalf("close after restart: shown=%d, hidden=%d", shown, hidden)
	}
}
