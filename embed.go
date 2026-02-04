package embedded

import (
	"embed"
	_ "embed"
)

// BasculaServicio contains the embedded service binary.
//
// NOTE: The embedded path "tmp/BasculaServicio.exe" must match the output
// location and filename configured in Taskfile.yml (or other build tooling).
// If the build changes the binary name or path, update this directive
// accordingly or embedding will fail at build time.
//
//go:embed tmp/BasculaServicio.exe
var BasculaServicio []byte

// WebFiles contains the static web assets (HTML, CSS, JS).
// This captures the 'internal/assets/web' directory recursively.
//
//go:embed internal/assets/web
var WebFiles embed.FS
