# 📊 Scale Daemon - Servicio Windows para Básculas Seriales con WebSocket

<div align="center">

<img src="img/pos.jpg" alt="POS Printer Logo" width="200" height="auto">

[![Go Version](https://img.shields.io/badge/Go-1.24.6-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Windows](https://img.shields.io/badge/Windows-go_SVC-0078D6?style=for-the-badge&logo=go)](https://github.com/judwhite/go-svc)
[![Taskfile](https://img.shields.io/badge/Taskfile-Automation-FF6347?style=for-the-badge&&logo=go)](https://taskfile.dev)
[![TUI](https://img.shields.io/badge/Bubbletea-TUI-FF69B4?style=for-the-badge&logo=go)](https://github.com/charmbracelet/bubbletea)

*Servicio Windows para lectura de básculas seriales con interfaz WebSocket*

</div>

---

## 📋 Descripción

Sistema para la lectura de básculas industriales a través de puerto serial (COM), que expone los datos mediante un
servidor WebSocket. Incluye dos modos de operación con instalador interactivo para Windows.

### 🎯 Características Principales

- ✅ **Servicio Windows nativo** - Se ejecuta en segundo plano
- ✅ **Comunicación WebSocket** - Integración fácil con aplicaciones web
- ✅ **Modo de prueba** - Simula datos para desarrollo
- ✅ **Instalador interactivo** - Gestión completa del servicio
- ✅ **Multi-ambiente** - Configuraciones para producción y desarrollo
- ✅ **Reconexión automática** - Manejo robusto de errores

---

## 🏗️ Arquitectura del Sistema

### 📊 Componentes Principales

#### `service.go` - Servicio WebSocket

| Componente               | Función               | Descripción                                  |
|--------------------------|-----------------------|----------------------------------------------|
| **Program**              | Controlador principal | Implementa interfaz `svc.Service` de Windows |
| **iniciarLectura()**     | Lector serial         | Lee datos del puerto COM continuamente       |
| **iniciarBroadcaster()** | Distribuidor          | Envía pesos a todos los clientes conectados  |
| **manejarCliente()**     | Handler WebSocket     | Gestiona conexiones individuales             |
| **Configuracion**        | Estructura de datos   | Puerto, marca, modo prueba, ambiente         |

#### `installer.go` - Instalador Interactivo

| Componente              | Función          | Descripción                              |
|-------------------------|------------------|------------------------------------------|
| **model**               | Estado TUI       | Maneja el estado de la interfaz terminal |
| **menuItem**            | Opciones de menú | Define acciones disponibles              |
| **installServiceCmd()** | Instalador       | Extrae y registra el servicio Windows    |
| **embedded**            | Binario embebido | Contiene el ejecutable del servicio      |

### Vista General

````mermaid
graph TB
    subgraph "Cliente Web"
        HTML[index_envs.html]
        JS[JavaScript Client]
    end

    subgraph "Servicio Windows"
        WS[WebSocket Server]
        SERIAL[Serial Reader]
        CONFIG[Configuración]
        BROADCAST[Broadcaster]
    end

    subgraph "Hardware"
        BASCULA[Báscula Serial]
        COM[Puerto COM]
    end

    subgraph "Instalador"
        TUI[Terminal UI]
        MANAGER[Service Manager]
        EMBEDDED[Servicio Embebido]
    end

    HTML --> JS
    JS <-->|ws://| WS
    WS --> BROADCAST
    SERIAL --> BROADCAST
    BROADCAST --> JS
    CONFIG <--> WS
    SERIAL <--> COM
    COM <--> BASCULA
    TUI --> MANAGER
    MANAGER -->|Instala| WS
    EMBEDDED -->|Extrae| WS
    style WS fill: #0080FF
    style TUI fill: #00FF40
    style BASCULA fill: #FFD700
````

### Flujo de Datos Principal

````mermaid
sequenceDiagram
    participant B as Báscula
    participant S as Servicio
    participant W as WebSocket
    participant C as Cliente Web

    loop Lectura Continua
        S ->> B: Comando "P"
        B ->> S: Peso (ej: "15.50")
        S ->> W: Broadcast peso
        W ->> C: Enviar a todos los clientes
        C ->> C: Actualizar UI
    end

    C ->> W: Cambiar configuración
    W ->> S: Aplicar config
    S ->> S: Reiniciar puerto serial
````

---

## 🛠️ Guía de Desarrollo

### 📦 Requisitos Previos

- **Windows 10/11** (64-bit)
- **Go 1.24.6**
- **Task** (gestor de tareas)

### 📁 Estructura del Proyecto

```
scale-daemon/
├── 📂 bin/                          # Binarios compilados
│   ├── BasculaServicio_Remoto.exe   # Servicio para conexión remota
│   ├── BasculaServicio_Local.exe    # Servicio para conexión local
│   ├── BasculaInstalador_Local.exe  # Instaladores
│   └── BasculaInstalador_Remoto.exe
│
├── 📂 cmd/                    # Código fuente principal
│   ├── 📂 BasculaServicio/
│   │   └── service.go         # Servicio WebSocket/Serial
│   └── 📂 BasculaInstalador/
│       └── installer.go       # Instalador interactivo con servicio embebido
│
├── 📂 init/                   # Archivos de inicialización
│   └── deps.bat               # Dependencias de Windows
│
├── 📂 web/                    # Archivos de inicialización
│   └── index_envs.bat         # Cliente web de prueba
│
├── 📄 embedded.go             # Serialización de binario
├── 📄 go.mod                  # Dependencias Go
├── 📄 go.sum                  # Checksums de dependencias
└── 📄 Taskfile.yml            # Automatización de tareas
```

### 🚀 Instalación de Dependencias

#### Opción 1: Script Automático (Recomendado)

```batch
# Ejecutar como Administrador
deps.bat
```

#### Opción 2: Instalación Manual

```batch
# 1. Instalar Go
winget install GoLang.Go

# 2. Instalar Task
winget install Task.Task

# 3. Clonar repositorio
git clone https://github.com/adcondev/scale-daemon.git
cd daemonize-example

# 4. Descargar dependencias Go
go mod download
go mod tidy

# 5. Ver tareas disponibles
task --list
```

### 🔨 Compilación

````mermaid
graph LR
    A[task build:all] --> B[Compila service.go]
    B --> C[Genera BasculaServicio_prod.exe]
    C --> D[Embebe en installer.go]
    D --> E[Genera BasculaInstalador_prod.exe]
    F[task build:test] --> G[Compila service.go TEST]
    G --> H[Genera BasculaServicio_test.exe]
    H --> I[Embebe en installer.go TEST]
    I --> J[Genera BasculaInstalador_test.exe]
    style A fill: #FF0040
    style F fill: #00FF40
````

#### Compilar para Producción

```batch
# Compila ambos binarios para producción (0.0.0.0:8765)
task build:all
```

#### Compilar para Test/Desarrollo

```batch
# Compila ambos binarios para test (localhost:8765)
task build:test
```

---

## 💻 Guía de Instalación (Usuario Final)

### 📥 Descarga e Instalación

1. **Descargar el instalador** según tu ambiente:
    - 🔴 **Producción/Remoto**: `BasculaInstalador_Remoto.exe`
    - 🟢 **Local/Test**: `BasculaInstalador_Local.exe`

2. **Ejecutar como Administrador** (clic derecho → "Ejecutar como administrador")

3. **Menú Principal del Instalador**:

````mermaid
stateDiagram-v2
    [*] --> Menu
    Menu --> Instalar: Seleccionar [+]
    Menu --> Verificar: Seleccionar [i]
    Menu --> Salir: Seleccionar [X]
    Instalar --> Confirmar
    Confirmar --> Procesando: [S]í
    Confirmar --> Menu: [N]o
    Procesando --> Resultado
    Resultado --> Menu: Enter
    Verificar --> Estado
    Estado --> Menu: Enter
````

### 🎯 Escenario de Instalación Típico

```
1. Ejecutar BasculaInstalador_prod.exe como Admin
2. Seleccionar [+] Instalar Servicio
3. Confirmar con 'S'
4. El servicio se instala en C:\Program Files\BasculaServicio\
5. Se inicia automáticamente
6. Verificar estado con opción [i]
```

### 🧪 Prueba del Servicio

1. **Abrir el archivo HTML de prueba**: `index_envs.html`
2. **Configurar parámetros**:
    - Puerto COM: `COM3` (ajustar según tu báscula)
    - Marca: `Rhino BAR 8RS`
    - Modo prueba: ☑️ (para simular datos)
3. **Aplicar configuración**
4. **Verificar recepción de datos** en el campo "Peso"
5. **Verificar logs del DevTools** presionando F12
6. **Probar las acciones del instalador** y verificar con Administrador de Tareas

### ⚙️ Configuración de Cliente Web

```javascript
// Ejemplo de conexión desde tu aplicación
// Cambiar a tu host si es remoto, 192.168.x.x, revisar con ipconfig y verificar firewall
const ws = new WebSocket('ws://localhost:8765');

ws.onmessage = (event) => {
    const peso = event.data;
    console.log('Peso recibido:', peso);
    // Actualizar tu UI aquí
};

// Enviar configuración
ws.send(JSON.stringify({
    tipo: "config",
    puerto: "COM3",
    marca: "Rhino BAR 8RS",
    modoPrueba: false
}));
```

---

## 🔧 Solución de Problemas

### 🚨 Problemas Comunes

| Problema                                   | Causa                      | Solución                                   |
|--------------------------------------------|----------------------------|--------------------------------------------|
| **"Permisos de Administrador Requeridos"** | No se ejecutó como admin   | Clic derecho → Ejecutar como administrador |
| **"Puerto COM no encontrado"**             | Puerto incorrecto o en uso | Verificar en Administrador de dispositivos |
| **"No se puede conectar al WebSocket"**    | Firewall bloqueando        | Agregar excepción para puerto 8765         |
| **Servicio no inicia**                     | Conflicto de puertos       | Verificar que el puerto 8765 esté libre    |
| **No recibe datos de báscula**             | Configuración incorrecta   | Verificar baudrate (9600) y comando ("P")  |

### 📂 Ubicaciones Importantes

```
# Binario del servicio
C:\Program Files\BasculaServicio\BasculaServicio_Remoto.exe     # Producción/Remoto
C:\Program Files\BasculaServicioTest\BasculaServicio_Local.exe  # Test/Local

# Archivos de log (solo en Test)
C:\ProgramData\BasculaServicioTest\BasculaServicio_Local.log

# Verificar servicio Windows
services.msc → Buscar "Servicio de Bascula"
Administrador de Tareas → Pestaña "Servicios" → Buscar "BasculaServicio_Remoto" o "BasculaServicio_Local"
```

### 🔍 Comandos Útiles de Diagnóstico

```batch
# Ver estado del servicio
sc query BasculaServicio

# Ver puertos COM disponibles
wmic path Win32_SerialPort get DeviceID,Description

# Verificar puerto 8765
netstat -an | findstr :8765

# Ver logs del servicio (solo Test)
type C:\ProgramData\BasculaServicioTest\BasculaServicio_Local.log
```

---

## 📈 Diferencias entre Ambientes

| Característica       | Producción (`prod`)                   | Test/Desarrollo (`test`)      |
|----------------------|---------------------------------------|-------------------------------|
| **Escucha en**       | `0.0.0.0:8765` (todas las interfaces) | `localhost:8765` (solo local) |
| **Nombre servicio**  | `BasculaServicio_Remoto`              | `BasculaServicio_Local`       |
| **Modo inicial**     | Real (lee puerto COM3)                | Prueba (simula datos)         |
| **Logs**             | Solo consola                          | Archivo + consola             |
| **Color instalador** | 🔴 Rojo/Azul                          | 🟢 Verde/Amarillo             |

---

## 👥 Contacto y Soporte

- **Empresa**: Red 2000, Mazatlán, Sinaloa
- **Dev**: [Adrián Constante](https://github.com/adcondev)
- **Año**: 2025
- **Versión**: Daemon v0.4.0 | Instalador v0.2.0

---

<div align="center">
<i> Desarrollado con ❤️ por RED 2000 </i>
</div>