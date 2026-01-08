//go:build test_build

package embedded

import _ "embed"

//go:embed embedded/bin/BasculaServicio.exe
var BasculaServicio []byte
