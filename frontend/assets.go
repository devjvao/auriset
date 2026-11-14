package frontend

import "embed"

// Assets contains the static files served by the desktop window.
//
//go:embed index.html app.js styles.css quick-presets.html quick-presets.js quick-presets.css
//go:embed preset-confirmation.html preset-confirmation.js preset-confirmation.css
//go:embed startup.js startup.css shortcut.js navigation.js
//go:embed equalizer-client.js response-chart.js
//go:embed credit-icons/*.png credit-icons/*.svg
var Assets embed.FS
