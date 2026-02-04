/* ==============================================================
   STATE - Scale Daemon
   ============================================================== */
const state = {
    socket: null,
    isConnected: false,
    lastWeight: '0.00',
    weightsReceived: 0,
    logCount: 0,
    startTime: Date.now(),
    lastWeightTime: null,
    pollTimer: null,
    config: {
        puerto: 'COM3',
        marca: 'Rhino BAR 8RS',
        modoPrueba: false,
        ambiente: 'unknown'
    }
};

/* ==============================================================
   DOM ELEMENTS
   ============================================================== */
let el = {};

function initElements() {
    el = {
        // Header
        connStatus: document.getElementById('connStatus'),
        uptimeVal: document.getElementById('uptimeVal'),
        weightsReceivedVal: document.getElementById('weightsReceivedVal'),
        lastWeightVal: document.getElementById('lastWeightVal'),

        // Weight Display
        weightContainer: document.getElementById('weightContainer'),
        weightDisplay: document.getElementById('weightDisplay'),
        weightUnit: document.getElementById('weightUnit'),
        weightStatus: document.getElementById('weightStatus'),
        weightStatusText: document.getElementById('weightStatusText'),

        // Metrics
        ambienteVal: document.getElementById('ambienteVal'),
        buildInfo: document.getElementById('buildInfo'),
        puertoVal: document.getElementById('puertoVal'),
        marcaVal: document.getElementById('marcaVal'),

        // Config Form
        puertoInput: document.getElementById('puertoInput'),
        marcaSelect: document.getElementById('marcaSelect'),
        modoPruebaCheck: document.getElementById('modoPruebaCheck'),
        btnApplyConfig: document.getElementById('btnApplyConfig'),

        // Log
        logContainer: document.getElementById('logContainer'),
        logCount: document.getElementById('logCount')
    };
}