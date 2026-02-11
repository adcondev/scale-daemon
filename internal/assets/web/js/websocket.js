/* ==============================================================
   WEBSOCKET - Scale Daemon
   PROTOCOL CONSTRAINTS (DO NOT MODIFY):
   - Inbound: 'config' message only
   - Outbound: 'ambiente' (once on connect), weight strings (streaming)
   - NO ping/pong/status via WebSocket (use HTTP endpoints)
   ============================================================== */

const ErrorDescriptions = {
    "ERR_EOF": "EOF recibido. Posible desconexión.",
    "ERR_TIMEOUT": "Timeout de lectura.",
    "ERR_READ": "Error de lectura.",
    "ERR_SCALE_CONN": "No se pudo conectar al puerto serial.",
};

function connectWebSocket() {
    addLog('INFO', `Conectando a ${CONFIG.WS_URL}...`);
    state.socket = new WebSocket(CONFIG.WS_URL);

    state.socket.onopen = () => {
        state.isConnected = true;
        state.startTime = Date.now();
        updateConnectionUI(true);
        addLog('INFO', '✅ Conectado al Scale Daemon');
        showToast('Conectado al servicio', 'success');

        // Start HTTP polling for metadata (not WebSocket)
        fetchHealth();
        state.pollTimer = setInterval(fetchHealth, CONFIG.POLL_INTERVAL);
    };

    state.socket.onclose = () => {
        state.isConnected = false;
        updateConnectionUI(false);

        // Stop HTTP polling
        if (state.pollTimer) {
            clearInterval(state.pollTimer);
            state.pollTimer = null;
        }

        addLog('ERROR', '❌ Conexión perdida. Reintentando...');
        setTimeout(connectWebSocket, CONFIG.RECONNECT_DELAY);
    };

    state.socket.onerror = () => {
        addLog('ERROR', '⚠️ Error de WebSocket');
    };

    state.socket.onmessage = (event) => {
        let msg;
        try {
            msg = JSON.parse(event.data);
        } catch (e) {
            // Failed to parse = RAW WEIGHT string (e.g., "12.50")
            handleWeightReading(event.data);
            return;
        }

        // Only 'ambiente' is a valid JSON control message
        // Everything else that parses as JSON but isn't 'ambiente' is treated as weight
        if (msg && typeof msg === 'object' && msg.tipo === 'ambiente') {
            handleAmbienteMessage(msg);
        } else {
            // Valid JSON but not 'ambiente' -> could be quoted weight string
            handleWeightReading(msg);
        }
    };
}

// Handle the 'ambiente' welcome message (PRESERVED - cannot rename)
function handleAmbienteMessage(msg) {
    addLog('INFO', `🌐 Ambiente: ${msg.ambiente} | Build: ${msg.version}`);
    updateEnvironmentDisplay(msg);

    if (msg.config) {
        updateConfigDisplay(msg.config);
    }
}

// Handle weight readings (format: raw string "12.50")
function handleWeightReading(peso) {
    const weight = String(peso).trim();

    // Check for error codes first
    if (weight.startsWith("ERR_")) {
        const errorMessage = ErrorDescriptions[weight] || `Error de lectura: ${weight}`;
        state.lastError = weight; // Store the error code
        updateConnectionUI(state.isConnected); // Update UI to show error
        addLog('ERROR', `⚠️ ${errorMessage}`, 'error');
        showToast(errorMessage, 'error');
        return;
    }

    // Handle normal weight readings
    if (weight && weight !== '') {
        // Clear any previous error on successful weight reading
        if (state.lastError) {
            state.lastError = null;
            updateConnectionUI(state.isConnected); // Update UI to clear error
        }

        updateWeightDisplay(weight);
        addLog('WEIGHT', `⚖️ ${weight} kg`, 'success');
        state.lastWeight = weight;
        state.weightsReceived++;
        if (el.weightsReceivedVal) {
            el.weightsReceivedVal.textContent = state.weightsReceived;
        }
    }
}

// Send message via WebSocket (only for 'config' type)
function sendMessage(msg) {
    if (!state.socket || !state.isConnected) {
        showToast('No conectado', 'error');
        return false;
    }
    state.socket.send(JSON.stringify(msg));
    return true;
}

// Send configuration update
function sendConfig() {
    const config = {
        tipo: 'config',
        puerto: el.puertoInput.value || 'COM3',
        marca: el.marcaSelect.value || 'Rhino BAR 8RS',
        modoPrueba: el.modoPruebaCheck.checked
    };

    if (sendMessage(config)) {
        addLog('SENT', `📤 Config: ${config.puerto} | ${config.marca} | Prueba: ${config.modoPrueba}`);
        showToast('Configuración enviada', 'info');
    }
}