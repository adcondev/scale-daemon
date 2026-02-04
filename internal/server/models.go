package server

// ConfigMessage matches the exact JSON structure from clients
// CONSTRAINT: All fields must match legacy format exactly
type ConfigMessage struct {
	Tipo       string `json:"tipo"`
	Puerto     string `json:"puerto"`
	Marca      string `json:"marca"`
	ModoPrueba bool   `json:"modoPrueba"`
	Dir        string `json:"dir,omitempty"`
}

// EnvironmentInfo sent to clients on connection
type EnvironmentInfo struct {
	Tipo     string          `json:"tipo"`
	Ambiente string          `json:"ambiente"`
	Version  string          `json:"version"`
	Config   ConfigForClient `json:"config"`
}

// ConfigForClient is the config subset sent to clients
type ConfigForClient struct {
	Tipo       string `json:"tipo,omitempty"`
	Puerto     string `json:"puerto"`
	Marca      string `json:"marca"`
	ModoPrueba bool   `json:"modoPrueba"`
	Dir        string `json:"dir"`
	Ambiente   string `json:"ambiente"`
}

// HealthResponse represents service health (excludes weight data per protocol)
type HealthResponse struct {
	Status string      `json:"status"`
	Scale  ScaleStatus `json:"scale"`
	Build  BuildInfo   `json:"build"`
	Uptime int         `json:"uptime_seconds"`
}

// ScaleStatus represents scale configuration state (no payload data)
type ScaleStatus struct {
	Connected bool   `json:"connected"`
	Port      string `json:"port"`
	Brand     string `json:"brand"`
	TestMode  bool   `json:"test_mode"`
}

// BuildInfo contains build metadata
type BuildInfo struct {
	Env  string `json:"env"`
	Date string `json:"date"`
	Time string `json:"time"`
}

// LogConfigMessage for verbose toggle
type LogConfigMessage struct {
	Tipo    string `json:"tipo"`
	Verbose bool   `json:"verbose"`
}

// LogTailMessage requests last N lines
type LogTailMessage struct {
	Tipo  string `json:"tipo"`
	Lines int    `json:"lines"`
}

// LogLinesResponse returns log lines
type LogLinesResponse struct {
	Tipo  string   `json:"tipo"`
	Lines []string `json:"lines"`
}

// LogFlushResult returned after flush operation
type LogFlushResult struct {
	Tipo  string `json:"tipo"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
