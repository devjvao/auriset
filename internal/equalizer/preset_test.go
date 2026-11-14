package equalizer

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestPresetDataReportsMatchKuba(t *testing.T) {
	// These digests came from running the shipped JavaScript encoder with
	// each preset's write bands and prepending the native HID report ID.
	fixtures := []struct{ id, digest string }{
		{"padrao", "47292bd0497470678a0f26b06c1b1c8ca058b4e5a65e69049473d504b6e6c164"},
		{"pref6", "b00924414aa51d0176aaa2396fcb1e782824550eab011f060c5291905a649289"},
		{"emv", "d20cf99310a59c78cd934ba1e0c9473cfc6529740e964fed24b445a16f1995ea"},
		{"basshead", "ac828735e3996c1f4f9cfd368a9061842283cca798101b1615580447bbda8eee"},
		{"energico", "dbad46eebc082de86f7c156cbd5896dfc425be2453a1ccca628ada3b2e9c4da7"},
		{"musical", "7e78b8b7de29c83a1f840ed9cfec9a7c38f246247ded22b8c75fc00ed9085538"},
		{"analitico", "c3bfe12dcb0a12cb3b3e5f52abe7e92f39a277eb9244e080509e5d0e3c135d4a"},
		{"referencia", "a4a9d450d7cc4e98a02f150d37379c09be15cecacf2233c6046de19a997e3a78"},
		{"harman", "128e80e1007713a343577649f4565f8900e83851701612d060d17d555e71526c"},
		{"vdsf", "941517c2b0b2106397539826fd4427faed37e58bea4a363ab00f042ba08ae3d1"},
		{"etymotic", "181c395156c590ba6cd0a8e01467fe56ac73d29ba4fce5d824315434c7d0680b"},
	}
	if len(presets) != len(fixtures) {
		t.Fatalf("got %d presets, want %d", len(presets), len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.id, func(t *testing.T) {
			p, ok := findPreset(fixture.id)
			if !ok {
				t.Fatalf("preset %s not found", fixture.id)
			}
			var data []byte
			for _, report := range p.dataReports() {
				data = append(data, report[:]...)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != fixture.digest {
				t.Errorf("data reports SHA-256 = %s, want %s", got, fixture.digest)
			}
		})
	}
}

func TestNaturalReportsMatchKuba(t *testing.T) {
	p, ok := findPreset("Natural")
	if !ok || p.id != "pref6" {
		t.Fatal("Natural alias did not resolve to pref6")
	}
	var stream reportRecorder
	if err := applyPreset(t.Context(), &stream, p); err != nil {
		t.Fatal(err)
	}

	// Generated offline from the shipped site's Natural bands, with the
	// report ID prepended to each payload for the native HID transport.
	const reportSize = 32
	const dataSize = 9 * reportSize
	if len(stream.sizes) != 10 {
		t.Fatalf("got %d HID writes, want 10", len(stream.sizes))
	}
	for i, size := range stream.sizes[:9] {
		if size != reportSize {
			t.Errorf("data report %d has %d bytes, want %d", i, size, reportSize)
		}
	}
	if stream.Len() != dataSize+6 {
		t.Fatalf("got %d bytes, want %d", stream.Len(), dataSize+6)
	}
	data := stream.Bytes()
	if got := fmt.Sprintf("%x", sha256.Sum256(data[:dataSize])); got != "b00924414aa51d0176aaa2396fcb1e782824550eab011f060c5291905a649289" {
		t.Errorf("data reports SHA-256 = %s", got)
	}
	if want := []byte{0xcb, 0x01, 0x65, 0x00, 0x01, 0xff}; !bytes.Equal(data[dataSize:], want) {
		t.Errorf("apply report = % x, want % x", data[dataSize:], want)
	}
}

type reportRecorder struct {
	bytes.Buffer
	sizes []int
}

func (w *reportRecorder) Write(data []byte) (int, error) {
	w.sizes = append(w.sizes, len(data))
	return w.Buffer.Write(data)
}

func TestApplyStopsOnShortWrite(t *testing.T) {
	p, ok := findPreset("natural")
	if !ok {
		t.Fatal("Natural preset missing")
	}
	w := new(shortWriter)
	if err := applyPreset(t.Context(), w, p); !errors.Is(err, io.ErrShortWrite) {
		t.Errorf("got error %v, want io.ErrShortWrite", err)
	}
	if w.calls != 1 {
		t.Errorf("got %d writes, want 1", w.calls)
	}
}

type shortWriter struct{ calls int }

func (w *shortWriter) Write(data []byte) (int, error) {
	w.calls++
	return len(data) - 1, nil
}
