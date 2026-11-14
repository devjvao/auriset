# Equalizador Kuba

A Windows desktop app for the Auri USB-C DSP cable. It previews the frequency response and target curves for all 11 built-in presets and sends the selected preset directly to the cable. The app can stay in the tray, start with Windows, and open a compact picker via a configurable global shortcut (default: `Ctrl+Alt+K`). The history shows the last successful write by this app; it does not read the active preset from the cable.

## Build and test

Use Go 1.26 or newer on Windows. The app uses the Microsoft Edge WebView2 Runtime. The frontend is bundled by Go, so a regular build does not need Node.js, the Wails CLI, or the icon generator.

```powershell
go test -tags production ./...
go vet -tags production ./...
go build -tags production -trimpath -ldflags='-H windowsgui' -o build/bin/kuba-equalizer.exe ./cmd/kuba-equalizer
```

[GitHub Actions](.github/workflows/ci.yml) runs the Windows build, vet, lint,
race tests, and vulnerability scan on pushes to `main` and pull requests
targeting `main`. To run the race tests locally, set `CGO_ENABLED=1` and use a
recent MinGW-w64 GCC.

The **Sobre** tab displays `0.1.0-dev` by default. Set `-X main.appVersion=0.1.0` alongside `-H windowsgui` to stamp a release build with its version. Regenerate the Windows icon resources with `go generate ./cmd/kuba-equalizer` after changing `cmd/kuba-equalizer/tray.png`.

For a Windows smoke test, open the built executable, check cable detection and the graph, then apply a preset from the main window and from the picker. Check that the picker restores focus and shows its confirmation, the tray and second launch reopen the app, and shortcut and apply history persist after **Sair** and relaunch. On a test account, enable Windows startup and sign out and back in to check that the app starts in the tray.

## Publish a Windows release

Tag the commit you want to release and push the tag to GitHub. For example, from an up-to-date `main`:

```powershell
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

The [release workflow](.github/workflows/release.yml) tests the app, stamps the tag version into **Sobre**, and publishes these Windows x64 assets, each with a `.sha256` checksum:

- `kuba-equalizer-setup-windows-amd64.exe`: Brazilian Portuguese installer. It installs for the current user without requiring admin rights, creates a Start menu shortcut, and runs the bundled Microsoft WebView2 bootstrapper only if the runtime is missing. The bootstrapper requires Internet access.
- `kuba-equalizer-windows-amd64.exe`: portable executable. Run it directly on a PC with the WebView2 Runtime installed.

To upgrade or uninstall, choose **Sair** from the tray menu first. The installer preserves settings and preset history. Uninstall removes the Windows startup entry only if it points to the installed copy. Tags like `v0.1.0-rc.1` publish prereleases. The portable executable and installer are currently unsigned.

## Project layout

| Path | Responsibility |
| --- | --- |
| `cmd/kuba-equalizer` | Windows entry point and Wails wiring: windows, tray, startup, single instance, global shortcut, and native icons. |
| `internal/equalizer` | Preset data, graph calculations, cable report encoding, write orchestration, and last-applied history. The Windows HID adapter finds the Auri and handles native report I/O. |
| `internal/config` | Shared application settings directory and file replacement. |
| `frontend` | Bundled HTML, CSS, and JavaScript for the main window, picker, and confirmation. |
| `installer` | Brazilian Portuguese Inno Setup definition for the Windows installer. |
| `tools/winicon` | Build-only Go module that generates the Windows icon resources from the tray artwork. |
| `build/bin` | Generated Windows release assets. |

The desktop wiring passes configuration paths and the Windows HID implementation to the equalizer. The frontend uses Wails bindings; preserve their method names and JSON fields when moving code. Plot curves and DSP reports are separate outputs, each covered by reference tests.

The app keeps the shortcut, tray notice acknowledgment, and last applied preset under the current user's `Kuba Equalizer` configuration directory. The opt-in Windows startup setting uses the current user's Windows login registration.
