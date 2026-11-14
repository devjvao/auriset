package main

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	windowsStartupArg   = "--windows-startup"
	windowsStartupKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	windowsStartupName  = "Kuba Equalizer"
	windowsStartupLimit = 260
)

// WindowsStartup manages the opt-in login launch for the current Windows user.
type WindowsStartup struct{}

// Enabled reports whether the application's login entry is registered.
func (*WindowsStartup) Enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsStartupKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open Windows startup key: %w", err)
	}
	defer func() { _ = key.Close() }()

	_, _, err = key.GetStringValue(windowsStartupName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Windows startup entry: %w", err)
	}
	return true, nil
}

// SetEnabled adds or removes only the application's own login entry.
func (*WindowsStartup) SetEnabled(enabled bool) error {
	if !enabled {
		return deleteWindowsStartup()
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate application executable: %w", err)
	}
	command, err := windowsStartupCommand(executable)
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsStartupKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Windows startup key for writing: %w", err)
	}
	defer func() { _ = key.Close() }()

	if err := key.SetStringValue(windowsStartupName, command); err != nil {
		return fmt.Errorf("register Windows startup entry: %w", err)
	}
	return nil
}

func deleteWindowsStartup() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsStartupKey, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open Windows startup key for writing: %w", err)
	}
	defer func() { _ = key.Close() }()

	if err := key.DeleteValue(windowsStartupName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("remove Windows startup entry: %w", err)
	}
	return nil
}

func windowsStartupCommand(executable string) (string, error) {
	command := windows.ComposeCommandLine([]string{executable, windowsStartupArg})
	encoded, err := windows.UTF16FromString(command)
	if err != nil {
		return "", fmt.Errorf("encode Windows startup command: %w", err)
	}
	if len(encoded)-1 > windowsStartupLimit { // Exclude the terminating NUL.
		return "", errors.New("application path is too long for Windows startup")
	}
	return command, nil
}

func isWindowsStartup(args []string) bool {
	// Wails forwards os.Args, including the executable name.
	return len(args) > 1 && slices.Contains(args[1:], windowsStartupArg)
}
