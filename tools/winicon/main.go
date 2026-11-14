// Command winicon generates the Windows icon resources from the tray artwork.
package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	dir := filepath.Join("..", "..", "cmd", "kuba-equalizer")
	data, err := os.ReadFile(filepath.Join(dir, "tray.png"))
	if err != nil {
		return err
	}
	artwork, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode tray artwork: %w", err)
	}
	icon, err := winres.NewIconFromResizedImage(artwork, []int{256, 64, 48, 32, 24, 16})
	if err != nil {
		return fmt.Errorf("convert app icon: %w", err)
	}

	var resources winres.ResourceSet
	// Wails uses icon group 3 for native Windows windows.
	if err := resources.SetIcon(winres.ID(3), icon); err != nil {
		return fmt.Errorf("set app icon: %w", err)
	}
	for _, arch := range []winres.Arch{winres.ArchAMD64, winres.ArchARM64, winres.ArchI386} {
		var object bytes.Buffer
		if err := resources.WriteObject(&object, arch); err != nil {
			return fmt.Errorf("create Windows %s resource object: %w", arch, err)
		}
		name := fmt.Sprintf("icon_windows_%s.syso", arch)
		if err := os.WriteFile(filepath.Join(dir, name), object.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}
