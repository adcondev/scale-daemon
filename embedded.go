package embedded

import _ "embed"

// BasculaServicio contains the embedded service binary.
// The Taskfile copies the correct variant (Local/Remote) to this path before building.
//
//go:embed embedded/bin/BasculaServicio.exe
var BasculaServicio []byte
