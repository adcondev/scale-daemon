/* ==============================================================
   UI FUNCTIONS - Scale Daemon
   ============================================================== */

function updateConnectionUI(connected) {
    el.connStatus.className = 'conn-badge ' + (connected ? 'online' : 'offline');
    el.connStatus.innerHTML = `<span class="conn-dot"></span><span>${connected ? 'En Línea' : 'Desconectado'}</span>`;

    if (el.btnApplyConfig) {
        el.btnApplyConfig.disabled = !connected;
    }
}

function updateWeightDisplay(weight) {
    if (el.weightDisplay) {
        el.weightDisplay.textContent = weight;

        // Pulse animation
        el.weightDisplay.classList.remove('pulse');
        void el.weightDisplay.offsetWidth; // Trigger reflow
        el.weightDisplay.classList.add('pulse');
    }

    // Update header last weight
    if (el.lastWeightVal) {
        el.lastWeightVal.textContent = `${weight} kg`;
    }

    // Update weight status
    if (el.weightStatus && el.weightStatusText) {
        el.weightStatus.classList.remove('idle');
        el.weightStatusText.textContent = 'Recibiendo datos...';
    }

    // Activate weight container
    if (el.weightContainer) {
        el.weightContainer.classList.add('active');
    }

    state.lastWeightTime = Date.now();
}

function updateEnvironmentDisplay(msg) {
    if (el.ambienteVal) {
        const ambiente = msg.ambiente || 'unknown';
        el.ambienteVal.textContent = ambiente;
        el.ambienteVal.className = 'metric-value ' +
            (ambiente.includes('TEST') || ambiente.includes('LOCAL') ? 'warning' : 'info');
    }

    if (el.buildInfo) {
        el.buildInfo.textContent = `Build: ${msg.version || '--'}`;
    }
}

function updateConfigDisplay(config) {
    state.config = { ...state.config, ...config };

    // Update form inputs
    if (el.puertoInput) el.puertoInput.value = config.puerto || 'COM3';
    if (el.marcaSelect) el.marcaSelect.value = config.marca || 'Rhino BAR 8RS';
    if (el.modoPruebaCheck) el.modoPruebaCheck.checked = config.modoPrueba || false;

    // Update metric cards
    if (el.puertoVal) el.puertoVal.textContent = config.puerto || 'COM3';
    if (el.marcaVal) el.marcaVal.textContent = config.marca || '--';
}

/* ==============================================================
   HTTP HEALTH POLLING (Diagnostics via HTTP, not WebSocket)
   ============================================================== */
async function fetchHealth() {
    try {
        const res = await fetch(CONFIG.HEALTH_URL);
        if (!res.ok) throw new Error('Health check failed');
        const data = await res.json();

        // Update uptime from server
        if (el.uptimeVal && data.uptime_seconds !== undefined) {
            el.uptimeVal.textContent = formatUptime(data.uptime_seconds);
        }

        // Update scale config status (no weight data per protocol)
        if (data.scale) {
            if (el.puertoVal) el.puertoVal.textContent = data.scale.port || '--';
            if (el.marcaVal) el.marcaVal.textContent = data.scale.brand || '--';
        }

        // Update build info
        if (data.build) {
            if (el.buildInfo) {
                el.buildInfo.textContent = `Build: ${data.build.date || '--'} ${data.build.time || ''}`;
            }
            if (el.ambienteVal) {
                el.ambienteVal.textContent = (data.build.env || 'unknown').toUpperCase();
            }
        }

    } catch (e) {
        console.warn('Health check failed:', e);
    }
}

/* ==============================================================
   UPTIME FORMATTING & STALE WEIGHT DETECTION
   ============================================================== */
function formatUptime(seconds) {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) return `${h}h ${m}m`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
}

function checkWeightStale() {
    if (state.lastWeightTime && el.weightStatus && el.weightStatusText) {
        const timeSinceLastWeight = Date.now() - state.lastWeightTime;
        if (timeSinceLastWeight > CONFIG.WEIGHT_STALE_TIMEOUT) {
            el.weightStatus.classList.add('idle');
            el.weightStatusText.textContent = 'Sin lecturas recientes';
            if (el.weightContainer) {
                el.weightContainer.classList.remove('active');
            }
        }
    }
}

// Periodic check for stale weights
setInterval(checkWeightStale, 2000);

/* ==============================================================
   LOGGING
   ============================================================== */
function addLog(type, message, textClass = '') {
    const time = new Date().toLocaleTimeString('es-MX', { hour12: false });
    const entry = document.createElement('div');
    entry.className = 'log-entry';

    const badgeClass = type.toLowerCase();
    const msgClass = textClass ? `log-message ${textClass}-text` : 'log-message';

    entry.innerHTML = `
        <span class="log-time">${time}</span>
        <span class="log-badge ${badgeClass}">${type}</span>
        <span class="${msgClass}">${escapeHtml(message)}</span>
    `;

    el.logContainer.appendChild(entry);
    el.logContainer.scrollTop = el.logContainer.scrollHeight;

    state.logCount++;
    if (el.logCount) {
        el.logCount.textContent = `${state.logCount} entradas`;
    }

    while (el.logContainer.children.length > CONFIG.MAX_LOGS) {
        el.logContainer.removeChild(el.logContainer.firstChild);
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

/* ==============================================================
   TOASTS
   ============================================================== */
function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;

    const icons = { success: '✅', error: '❌', warning: '⚠️', info: 'ℹ️' };
    toast.innerHTML = `
        <span class="toast-icon">${icons[type]}</span>
        <span class="toast-message">${escapeHtml(message)}</span>
    `;

    container.appendChild(toast);

    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(100px)';
        setTimeout(() => toast.remove(), 300);
    }, 3500);
}