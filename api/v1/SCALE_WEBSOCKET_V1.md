# Scale Daemon - WebSocket API v1.0

## Índice
* [Descripción General](#descripción-general)
* [Endpoints](#endpoints)
    * [Configuración por Ambiente](#configuración-por-ambiente)
* [Protocolo WebSocket](#protocolo-websocket)
    * [Ciclo de Vida de Conexión](#ciclo-de-vida-de-conexión)
* [Mensajes del Cliente → Servidor](#mensajes-del-cliente--servidor)
    * [1. `config` - Actualizar Configuración](#1-config---actualizar-configuración)
* [Mensajes del Servidor → Cliente](#mensajes-del-servidor--cliente)
    * [1. `ambiente` - Información Inicial](#1-ambiente---información-inicial)
    * [2. Streaming de Peso (String Puro)](#2-streaming-de-peso-string-puro)
    * [3. Códigos de Error (Broadcasting)](#3-códigos-de-error-broadcasting)
    * [4. Códigos de Error (Control y Configuración)](#4-códigos-de-error-control-y-configuración)
* [HTTP Endpoints](#http-endpoints)
    * [GET `/health`](#get-health)
    * [GET `/ping`](#get-ping)
* [Implementación de Cliente (Ejemplo JS)](#implementación-de-cliente-ejemplo-js)

## Descripción General

Scale Daemon expone un servidor WebSocket optimizado para la transmisión de datos de peso en tiempo real (streaming) y
la gestión de configuración del servicio.

A diferencia de otros protocolos puramente basados en objetos JSON, este servicio utiliza un enfoque híbrido:

1. **Control y Metadatos:** Objetos JSON estructurados.
2. **Streaming de Peso:** Strings JSON simples para minimizar overhead y latencia.

---

## Endpoints

| Protocolo | Endpoint                    | Descripción                     |
|-----------|-----------------------------|---------------------------------|
| WebSocket | `ws://{host}:8765/ws`       | Canal de datos y configuración  |
| HTTP GET  | `http://{host}:8765/health` | Health check y diagnóstico      |
| HTTP GET  | `http://{host}:8765/ping`   | Verificación de latencia simple |
| HTTP GET  | `http://{host}:8765/`       | Dashboard visual (HTML)         |

### Configuración por Ambiente

| Ambiente   | Host Bind   | Puerto | Servicio Windows             |
|------------|-------------|--------|------------------------------|
| **Remoto** | `0.0.0.0`   | 8765   | `R2k_BasculaServicio_Remote` |
| **Local**  | `localhost` | 8765   | `R2k_BasculaServicio_Local`  |

---

## Protocolo WebSocket

### Ciclo de Vida de Conexión

```text
┌─────────────────────────────────────────────────────────────────┐
│  1. Cliente conecta a ws://host:8765/ws                         │
│  2. Servidor envía mensaje "ambiente" (Configuración inicial)   │
│  3. STREAMING ACTIVO:                                           │
│     Servidor -> "12.50" (Peso)                                  │
│     Servidor -> "12.55"                                         │
│     Servidor -> "ERR_TIMEOUT" (Si hay error)                    │
│  4. Cliente puede enviar comando "config" para cambiar puerto   │
│  5. Servidor reinicia driver y reanuda streaming                │
└─────────────────────────────────────────────────────────────────┘

```

---

## Mensajes del Cliente → Servidor

### 1. `config` - Actualizar Configuración

Modifica los parámetros de conexión con la báscula en caliente. Requiere un token de autorización si el servidor fue
compilado con seguridad habilitada.

**Estructura:**

```json
{
  "tipo": "config",
  "puerto": "COM3",
  "marca": "Rhino BAR 8RS",
  "modoPrueba": false,
  "auth_token": "tu-token-de-seguridad"
}
```

| Campo        | Tipo    | Requerido | Descripción                                                                       |
|--------------|---------|-----------|-----------------------------------------------------------------------------------|
| `tipo`       | string  | ✓         | Debe ser `"config"`                                                               |
| `puerto`     | string  | ✓         | Puerto serial (`COM1`, `COM3`, `/dev/ttyUSB0`)                                    |
| `marca`      | string  | ✓         | Marca de la báscula (`Rhino BAR 8RS`, `rhino`)                                    |
| `modoPrueba` | boolean | ✓         | `true` para generar pesos simulados, `false` real                                 |
| `auth_token` | string  | ✓*        | Token de autenticación para autorizar cambios (Requerido si el backend lo exige). |

---

## Mensajes del Servidor → Cliente

### 1. `ambiente` - Información Inicial

Enviado inmediatamente después de que el cliente conecta.

```json
{
  "tipo": "ambiente",
  "ambiente": "REMOTE",
  "version": "2026-02-11 14:00:00",
  "config": {
    "puerto": "COM3",
    "marca": "Rhino BAR 8RS",
    "modoPrueba": false,
    "ambiente": "REMOTE"
  }
}

```

### 2. Streaming de Peso (String Puro)

Para máxima eficiencia, las lecturas de peso NO se envuelven en un objeto. Se envían como un string JSON directo.

**Ejemplo:**

```json
"15.40"

```

### 3. Códigos de Error (Broadcasting)

Los errores críticos se envían a través del mismo canal de streaming, prefijados con `ERR_`.

**Códigos:**

| Código           | Causa                                   |
|------------------|-----------------------------------------|
| `ERR_SCALE_CONN` | No se puede abrir el puerto serial      |
| `ERR_EOF`        | Cable desconectado (EOF)                |
| `ERR_TIMEOUT`    | Báscula no responde (5s timeout)        |
| `ERR_READ`       | Error general de lectura (ruido/driver) |

**Ejemplo:**

```json
"ERR_SCALE_CONN"

```

### 4. Códigos de Error (Control y Configuración)

Cuando falla una operación enviada por el cliente (por ejemplo, al intentar cambiar la configuración), el servidor
responde con un objeto JSON estructurado, no con un string de streaming.

**Estructura:**

```json
{
  "tipo": "error",
  "error": "AUTH_INVALID_TOKEN"
}

```

Código,Causa
AUTH_INVALID_TOKEN,El auth_token proporcionado en el mensaje config es incorrecto o está ausente.
RATE_LIMITED,Se ha excedido el límite de cambios de configuración (máximo 15 por minuto por cliente).

---

## HTTP Endpoints

### GET `/health`

Endpoint de monitoreo para health checks.

**Response:**

```json
{
  "status": "ok",
  "scale": {
    "connected": true,
    "port": "COM3",
    "brand": "Rhino BAR 8RS",
    "test_mode": false
  },
  "build": {
    "env": "remote",
    "date": "2026-02-11",
    "time": "10:30:00"
  },
  "uptime_seconds": 3600
}

```

### GET `/ping`

Verificación de latencia mínima.

**Response:** `pong` (text/plain)

---

## Implementación de Cliente (Ejemplo JS)

El cliente debe distinguir entre objetos JSON (mensajes de control) y Strings (peso/error).

```javascript
ws.onmessage = (event) => {
    let msg;
    try {
        msg = JSON.parse(event.data);
    } catch (e) {
        // Si no es JSON válido, ignorar o manejar como raw
        return;
    }

    // 1. Es un Objeto de Control (tiene propiedad "tipo")
    if (msg && typeof msg === 'object' && msg.tipo === 'ambiente') {
        console.log("Configuración recibida:", msg);
        return;
    }

    // 2. Es un String de Peso o Error (String JSON)
    // wsjson.Write en Go envía strings entre comillas: "12.50"
    if (typeof msg === 'string') {
        if (msg.startsWith("ERR_")) {
            console.error("Error de Báscula:", msg); // e.g., ERR_SCALE_CONN
        } else {
            console.log("Peso:", msg); // e.g., 12.50
        }
    }
};

```