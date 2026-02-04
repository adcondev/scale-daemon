/* ==============================================================
   MAIN - Scale Daemon Dashboard
   ============================================================== */

document.addEventListener('DOMContentLoaded', () => {
    initElements();
    init();
});

function init() {
    connectWebSocket();
    addLog('INFO', '🚀 Dashboard inicializado');
    setupEventListeners();
}

function setupEventListeners() {
    // Apply Configuration (WebSocket)
    if (el.btnApplyConfig) {
        el.btnApplyConfig.addEventListener('click', sendConfig);
    }

    // HTTP Ping Button
    const btnPing = document.getElementById('btnPing');
    if (btnPing) {
        btnPing.addEventListener('click', async () => {
            const start = Date.now();
            try {
                const res = await fetch(CONFIG.PING_URL);
                if (res.ok) {
                    const ms = Date.now() - start;
                    addLog('PONG', `🏓 HTTP Pong: ${ms}ms`, 'success');
                    showToast(`Ping: ${ms}ms`, 'success');
                } else {
                    throw new Error(res.statusText);
                }
            } catch (e) {
                addLog('ERROR', `Ping falló: ${e.message}`);
                showToast('Ping falló', 'error');
            }
        });
    }

    // HTTP Refresh/Status Button
    const btnRefresh = document.getElementById('btnRefresh');
    if (btnRefresh) {
        btnRefresh.addEventListener('click', async function () {
            const btn = this;
            btn.classList.add('spin-anim');
            btn.disabled = true;

            try {
                await fetchHealth();
                showToast('Estado actualizado', 'success');
            } catch (e) {
                showToast('Error de conexión', 'error');
            } finally {
                setTimeout(() => {
                    btn.classList.remove('spin-anim');
                    btn.disabled = false;
                }, 500);
            }
        });
    }

    // Clear Logs (dashboard only)
    const btnClearLogs = document.getElementById('btnClearLogs');
    if (btnClearLogs) {
        btnClearLogs.addEventListener('click', () => {
            el.logContainer.innerHTML = '';
            state.logCount = 0;
            if (el.logCount) el.logCount.textContent = '0 entradas';
            showToast('Logs limpiados', 'info');
        });
    }

    // Export Logs
    const btnExportLogs = document.getElementById('btnExportLogs');
    if (btnExportLogs) {
        btnExportLogs.addEventListener('click', () => {
            const entries = el.logContainer.querySelectorAll('.log-entry');
            const lines = Array.from(entries).map(e => {
                const time = e.querySelector('.log-time').textContent;
                const type = e.querySelector('.log-badge').textContent;
                const msg = e.querySelector('.log-message').textContent;
                return `[${time}] ${type}: ${msg}`;
            });

            const blob = new Blob([lines.join('\n')], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `scale-daemon-${Date.now()}.log`;
            a.click();
            URL.revokeObjectURL(url);
            showToast('Logs exportados', 'success');
        });
    }

    // Test Mode Toggle (visual feedback)
    if (el.modoPruebaCheck) {
        el.modoPruebaCheck.addEventListener('change', () => {
            const status = el.modoPruebaCheck.checked ? 'ON' : 'OFF';
            addLog('INFO', `🧪 Modo prueba: ${status}`);
        });
    }
}