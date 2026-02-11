package embedded

import (
	"embed"
	_ "embed"
)

// WebFiles contains the static web assets (HTML, CSS, JS).
// This captures the 'internal/assets/web' directory recursively.
//
//go:embed internal/assets/web
var WebFiles embed.FS
