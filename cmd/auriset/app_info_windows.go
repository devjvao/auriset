package main

// Release builds can set this with -ldflags "-X main.appVersion=0.1.0".
var appVersion = "0.1.0-dev"

// AppInfo provides build information to the desktop frontend.
type AppInfo struct{}

// Version returns the application version embedded in the build.
func (*AppInfo) Version() string {
	return appVersion
}
