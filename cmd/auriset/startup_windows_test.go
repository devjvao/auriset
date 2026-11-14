package main

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsStartupCommand(t *testing.T) {
	executable := `C:\Program Files\Áudio Kuba\auriset.exe`
	command, err := windowsStartupCommand(executable)
	if err != nil {
		t.Fatal(err)
	}
	args, err := windows.DecomposeCommandLine(command)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{executable, "--windows-startup"}; !slices.Equal(args, want) {
		t.Fatalf("startup command %q parsed as %q, want %q", command, args, want)
	}
	if !isWindowsStartup(args) {
		t.Fatalf("startup command was not recognized: %q", args)
	}
}

func TestWindowsStartupCommandRejectsLongPaths(t *testing.T) {
	// Count UTF-16 characters as Windows does, not UTF-8 bytes.
	if _, err := windowsStartupCommand(`C:\` + strings.Repeat("á", 115) + ".exe"); err != nil {
		t.Fatalf("a path under the Windows limit was rejected: %v", err)
	}
	if _, err := windowsStartupCommand(`C:\` + strings.Repeat("a", 260) + ".exe"); err == nil {
		t.Fatal("an overlong command should be rejected before registry registration")
	}
}

func TestIsWindowsStartup(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "manual launch", args: []string{"auriset.exe"}},
		{name: "login launch", args: []string{"auriset.exe", "--windows-startup"}, want: true},
		{name: "executable name alone", args: []string{"--windows-startup"}},
		{name: "not the exact flag", args: []string{"auriset.exe", "--windows-startup=no"}},
		{name: "no args"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWindowsStartup(tt.args); got != tt.want {
				t.Errorf("isWindowsStartup(%q) = %t, want %t", tt.args, got, tt.want)
			}
		})
	}
}
