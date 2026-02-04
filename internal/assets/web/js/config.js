/* ==============================================================
   CONFIGURATION - Scale Daemon
   Protocol: WebSocket for DATA, HTTP for DIAGNOSTICS
   ============================================================== */
const CONFIG = {
    // WebSocket: Weight data (out) + Config (in)
    WS_URL: `ws://${window.location.hostname}:${window.location.port || 8765}/ws`,

    // HTTP: Diagnostics only (no payload data)
    HEALTH_URL: `http://${window.location.hostname}:${window.location.port || 8765}/health`,
    PING_URL: `http://${window.location.hostname}:${window.location.port || 8765}/ping`,

    RECONNECT_DELAY: 3000,
    POLL_INTERVAL: 5000,
    MAX_LOGS: 200,
    WEIGHT_STALE_TIMEOUT: 10000
};

// Scale brands available for configuration
const SCALE_BRANDS = [
    { value: 'Rhino BAR 8RS', label: 'Rhino BAR 8RS' },
    { value: 'rhino', label: 'Rhino (Generic)' }
];

// Common COM ports
const COM_PORTS = [
    'COM1', 'COM2', 'COM3', 'COM4',
    'COM5', 'COM6', 'COM7', 'COM8'
];