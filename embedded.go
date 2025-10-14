package embedded

import _ "embed"

// Distintos binarios para distintos entornos.
// Como se crean binarios separados, se embeben por separado para cada instalador.

//go:embed bin/BasculaServicio_prod.exe
var BasculaServicioProd []byte

//go:embed bin/BasculaServicio_test.exe
var BasculaServicioTest []byte
