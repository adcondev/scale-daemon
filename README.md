# ⚖️ Scale Daemon

**Scale Daemon** es un servicio de Windows de alto rendimiento diseñado para actuar como puente entre básculas industriales (RS232/Serial) y aplicaciones web modernas. A diferencia de soluciones simples, este daemon funciona como un middleware persistente que gestiona la reconexión automática, el filtrado de ruido y la distribución de datos mediante WebSockets de baja latencia.

El servicio está optimizado para entornos de **retail y logística**, permitiendo que cualquier navegador en la red local obtenga lecturas de peso en tiempo real sin necesidad de drivers adicionales en el cliente.

---

## 🏗️ Arquitectura del Sistema

El daemon utiliza un modelo de **Broadcaster asíncrono**. Un lector serial dedicado (Producer) alimenta un canal central, el cual distribuye los datos a todos los clientes WebSocket conectados (Consumers) de forma concurrente.

### Estructura de Componentes

```mermaid
graph TD
    classDef go fill:#e1f5fe,stroke:#01579b,stroke-width:2px,color:#000;
    classDef data fill:#fff3e0,stroke:#e65100,stroke-width:2px,color:#000;
    classDef hw fill:#f3e5f5,stroke:#4a148c,stroke-width:2px,color:#000;

    subgraph Host["Windows Service Host"]
        direction TB
        Service[Wrapper svc.Service]:::go -->|Init/Start| HTTP[Servidor HTTP/WS]:::go
        Service -->|Start| Reader[Serial Reader Loop]:::go
        Reader -->|Canal| Broadcast[Broadcaster Engine]:::go
    end

    subgraph Hardware["Capa Física"]
        Scale[Báscula Industrial]:::hw -->|RS232/Serial| Reader
    end

    subgraph Network["Distribución"]
        Broadcast -->|Fan-Out| Client1[Web POS 1]:::data
        Broadcast -->|Fan-Out| Client2[Web POS 2]:::data
        Broadcast -->|Fan-Out| ClientN[Dashboard/Apps]:::data
    end

```

### Modelo de Concurrencia y Hot-Reload

El servicio implementa un sistema de **configuración en caliente**. Al recibir un mensaje de configuración vía WebSocket, el daemon detiene de forma segura la goroutine de lectura actual, cierra el puerto serial y reinicia el bucle con los nuevos parámetros (Puerto, Marca o Modo Prueba) sin interrumpir las conexiones de otros clientes.

```mermaid
sequenceDiagram
    participant C as Cliente Web
    participant S as WebSocket Server
    participant R as Serial Reader
    participant H as Hardware (COM)

    Note over R, H: Bucle de lectura activo
    C ->> S: JSON {"tipo":"config", "puerto":"COM4"}
    S ->> R: Señal de Reinicio (Mutex Lock)
    R ->> H: Close Port
    Note over R: Actualizando Configuración
    R ->> H: Open Port (COM4)
    R -->> S: OK / Reanudado
    S -->> C: Update Success

```

---

## 🚀 Características Principales

* 🔌 **Abstracción de Hardware**: Soporte para múltiples protocolos de básculas (Rhino, etc.) mediante comandos de solicitud de peso (`P`).
* 🔄 **Resiliencia Automática**: Estrategia de reintento con backoff ante desconexiones físicas del cable serial.
* 🧪 **Modo Simulación Integrado**: Generación de pesos aleatorios con fluctuación realista para desarrollo sin hardware físico.
* 🛠️ **Instalador Profesional**: TUI (Text User Interface) para gestionar el ciclo de vida del servicio (Instalar, Iniciar, Detener).
* 📊 **Dashboard de Diagnóstico**: Interfaz web embebida para monitorear el peso y probar la configuración visualmente.
* 🚨 **Diagnóstico en Tiempo Real**: Notificación inmediata de errores de conexión (puerto no encontrado, desconexión
  física) directamente en el Dashboard, eliminando la necesidad de revisar logs del servidor.

---

## 📡 Protocolo WebSocket

### Endpoints

* **WebSocket**: `ws://[IP]:8765/ws` (Conexión para recibir datos de peso)
* **Dashboard**: `http://[IP]:8765/` (Interfaz visual embebida)

### Intercambio de Datos

**1. Mensaje de Ambiente (Servidor -> Cliente):**
Al conectar, el servidor envía información del entorno inyectada durante la compilación.

```json
{
  "tipo": "ambiente",
  "ambiente": "REMOTE",
  "version": "2026-01-29 14:08:03",
  "config": {
    "puerto": "COM3",
    "marca": "Rhino BAR 8RS",
    "modoPrueba": false
  }
}

```

**2. Lectura de Peso (Broadcasting):**
El peso se envía como un string simple o JSON dependiendo de la estabilidad de la trama.

```json
"15.42"

```

---

### Códigos de Error

Además del peso, el servidor puede enviar códigos de error críticos para que el cliente notifique al usuario
visualmente.

| Código         | Descripción       | Causa Común                                                               |
|----------------|-------------------|---------------------------------------------------------------------------|
| ERR_SCALE_CONN | Error de Conexión | El puerto COM no existe o está ocupado por otro proceso.                  |
| ERR_EOF        | Desconexión (EOF) | El cable de la báscula fue desconectado físicamente durante la operación. |
| ERR_TIMEOUT    | Tiempo de Espera  | La báscula está conectada pero no responde a los comandos (5s).           |
| ERR_READ       | Error de Lectura  | Ruido en la línea o fallo del driver serial.                              |

## ⚙️ Configuración y Build

El proyecto utiliza un `Taskfile` para gestionar compilaciones inyectando variables en tiempo de enlace (`ldflags`).

| Tarea                | Descripción                                             |
|----------------------|---------------------------------------------------------|
| `task build:local`   | Compila instalador para entorno de pruebas (localhost). |
| `task build:remote`  | Compila instalador para producción (0.0.0.0).           |
| `task build:console` | Genera ejecutables de consola para debugging rápido.    |
| `task clean`         | Limpia binarios y archivos temporales.                  |

### Inyección de Metadatos

El proceso de build inyecta automáticamente:

* `BuildEnvironment`: Define el alcance de la red.
* `ServiceName`: Nombre del servicio en el SCM de Windows.
* `BuildDate/Time`: Estampa de tiempo de la versión.

---

## 📂 Estructura del Proyecto

```text
scale-daemon/
├── cmd/
│   ├── BasculaServicio/    # Código principal del Daemon
│   └── BasculaInstalador/  # TUI para instalación del servicio
├── internal/
│   └── assets/             # Recursos web (HTML/JS) embebidos
├── tmp/                    # Binarios temporales para el empaquetado
├── bin/                    # Artefactos finales (Instaladores)
├── embedded.go             # Implementación de go:embed
└── Taskfile.yml            # Automatización de tareas

```

---

## 📝 Logs y Mantenimiento

Los logs se almacenan en `%PROGRAMDATA%` con un sistema de **autorrotación** para prevenir el llenado del disco:

* **Ruta**: `C:\ProgramData\R2k_Bascula_Remote\R2k_Bascula_Remote.log`
* **Límite**: 5 MB (al excederse, se conservan las últimas 1000 líneas para trazabilidad).

---

## 🔐 Seguridad

Scale Daemon implementa un modelo de seguridad por capas, diseñado para entornos de retail donde se necesita
proteger la configuración del servicio sin impactar la lectura de peso en tiempo real.

### Capas de Protección

| Capa                | Protege                                | Mecanismo                                      |
|---------------------|----------------------------------------|------------------------------------------------|
| **Dashboard Login** | Acceso al panel de control (`/`)       | Contraseña + sesión con cookie HttpOnly        |
| **Config Token**    | Cambios de configuración vía WebSocket | Token de autorización en cada mensaje `config` |
| **Rate Limiter**    | Abuso de configuración                 | Máximo 3 cambios por minuto por conexión       |
| **Brute Force**     | Ataques de fuerza bruta al login       | Bloqueo de IP tras 5 intentos fallidos (5 min) |

### Modelo de Acceso por Endpoint

```text
┌─────────────────────────────────────────────────────────────────┐
│  PÚBLICO (sin autenticación)                                    │
│  ├── GET  /login          Página de login                       │
│  ├── POST /auth/login     Procesar login                        │
│  ├── GET  /ping           Verificación de latencia              │
│  ├── GET  /health         Diagnóstico del servicio              │
│  ├── WS   /ws             Streaming de peso + config (token)    │
│  ├── GET  /css/*          Archivos estáticos                    │
│  └── GET  /js/*           Archivos estáticos                    │
│                                                                 │
│  PROTEGIDO (sesión requerida)                                   │
│  └── GET  /               Dashboard (inyecta config token)      │
└─────────────────────────────────────────────────────────────────┘
```

> **Nota:** El endpoint `/ws` es público para permitir que aplicaciones POS reciban peso sin necesidad de
> autenticarse en el dashboard. Los cambios de configuración dentro del WebSocket están protegidos por el
> `authToken`, que sólo está disponible para sesiones autenticadas a través del dashboard.

### Configuración

Los secretos se definen en un archivo `.env` en el directorio del build system (`poster-tuis/`):

```env
# ⚠️ NO commitear a control de versiones
DASHBOARD_PASSWORD=MiContraseña2026
CONFIG_AUTH_TOKEN=mi-token-secreto
```

| Variable             | Vacío =                                          | Descripción                                        |
|----------------------|--------------------------------------------------|----------------------------------------------------|
| `DASHBOARD_PASSWORD` | Auth deshabilitado (acceso directo al dashboard) | Contraseña para el login del dashboard             |
| `CONFIG_AUTH_TOKEN`  | Config sin validación de token                   | Token requerido en mensajes `config` vía WebSocket |

### Pipeline de Inyección

```text
.env (plaintext) 
  → hashpw (bcrypt + base64) 
    → ldflags -X PasswordHashB64=... 
      → binario compilado (sin plaintext)
```

La contraseña **nunca** se almacena en texto plano en el binario. Se inyecta como un hash bcrypt codificado
en base64 mediante `ldflags` durante la compilación. El token de configuración se inyecta directamente
(no es un secreto criptográfico, es un valor de autorización).

### Sesiones

- Duración: **15 minutos** (configurable en `auth.go`)
- Cookie: `sd_session`, `HttpOnly`, `SameSite=Strict`
- Almacenamiento: en memoria del proceso (se pierden al reiniciar el servicio)
- Limpieza automática: goroutine periódica cada 5 minutos

### Auditoría

Todos los eventos de seguridad se registran con el prefijo `[AUDIT]`:

```
[AUDIT] LOGIN_SUCCESS | IP=192.168.1.100:54321
[AUDIT] LOGIN_FAILED | IP=192.168.1.100:54322
[AUDIT] LOGIN_BLOCKED | IP=192.168.1.100:54323 | reason=lockout
[AUDIT] CONFIG_ACCEPTED | puerto=COM4 marca=Rhino modoPrueba=false
[AUDIT] CONFIG_REJECTED | reason=invalid_token | puerto=COM4 marca=Rhino
[AUDIT] CONFIG_RATE_LIMITED | client=0xc0001a2000
```
