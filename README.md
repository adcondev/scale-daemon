# 🔧 BasculaServicio - Guía del Desarrollador

## Arquitectura

El proyecto consta de dos componentes principales:

### 1. BasculaServicio (Servicio Windows)

- **Ubicación**: `cmd/BasculaServicio/main.go`
- **Función**: Servicio Windows que maneja comunicación serial y WebSocket
- **Puerto**: 8765 (WebSocket)

### 2. BasculaInstalador (Instalador TUI)

- **Ubicación**: `cmd/BasculaInstalador/main.go`
- **Función**: Instalador interactivo con BubbleTea
- **Embebe**: El ejecutable del servicio

## Comandos de Desarrollo

```bash
# Instalar Task (una sola vez)
winget install Task.Task

# Ver tareas disponibles
task --list

# Compilar todo
task build:all

# Solo el servicio para probar por serparado
task service:build

# Solo el instalador
task installer:build

# Limpiar compilados
task clean
```

## Tecnologías Utilizadas

- **[Go 1.24.7]()**: Lenguaje principal
- **[BubbleTea]()**: Framework TUI para el instalador
- **[go-svc]()**: Integración con Windows Service
- **[WebSocket]()**: Comunicación en tiempo real
- **[go.bug.st/serial]()**: Comunicación con puerto serial

## Flujo de Embebido

1. Se compila `BasculaServicio.exe`
2. El instalador lo embebe con `//go:embed`
3. Al instalar, se extrae a `%ProgramFiles%`
4. Se registra como servicio Windows

## Debugging

Para depurar el servicio sin instalarlo:

```bash
PENDIENTE=1
task service:run

task service:logs

task service:stop
```

# 📊 BasculaServicio - Manual de Usuario

## ¿Qué es BasculaServicio?

BasculaServicio es un programa que permite conectar su báscula al computador y compartir las lecturas de peso a través
de su navegador web.

## Requisitos

- ✅ Windows 10 o superior
- ✅ Permisos de Administrador para la instalación
- ✅ Puerto COM disponible (para báscula física)

## Instalación Rápida

### Paso 1: Descargar

Descargue el archivo `BasculaInstalador.exe`

### Paso 2: Ejecutar como Administrador

1. Haga **clic derecho** sobre el archivo
2. Seleccione **"Ejecutar como administrador"**
3. Si Windows pregunta, haga clic en **"Sí"**

### Paso 3: Usar el Instalador

El instalador mostrará un menú interactivo:

```
╔══════════════════════════════════════════╗
║  Instalador del Servicio de Báscula      ║
╚══════════════════════════════════════════╝

📊 Estado Actual: ❌ NO INSTALADO

▸ 📦 Instalar Servicio
🗑️ Desinstalar Servicio
▶️ Iniciar Servicio
⏹️ Detener Servicio
📊 Ver Estado
📝 Ver Logs
❌ Salir

⌨️ ↑/↓: Navegar • Enter: Seleccionar • Q: Salir
```

### Paso 4: Instalar

1. Use las **flechas ↑↓** para seleccionar **"📦 Instalar Servicio"**
2. Presione **Enter**
3. Confirme con **S** cuando se le pregunte
4. Espere a que termine la instalación

## Uso del Servicio

Una vez instalado, el servicio se ejecuta automáticamente.

### Para ver el peso en su navegador:

1. Abra su navegador web (Chrome, Firefox, Edge)
2. Vaya a la carpeta donde tiene los archivos HTML
3. Abra `index_config.html` para configurar
4. Abra `index.html` para ver el peso

### Configuración de la Báscula:

En `index_config.html` puede configurar:

- **Puerto COM**: Ej: COM3, COM4, COM6
- **Marca**: rhino (u otra compatible)
- **Modo prueba**: Active para simular pesos sin báscula

## Solución de Problemas

### El servicio no inicia

- Verifique que el puerto COM sea el correcto
- Asegúrese que la báscula esté conectada y encendida

### No veo el peso en el navegador

- Verifique que el servicio esté en ejecución
- Actualice la página del navegador (F5)
- Active el modo prueba para verificar la conexión

### Error de permisos

- Siempre ejecute el instalador como Administrador

## Desinstalación

1. Ejecute el instalador como Administrador
2. Seleccione **"🗑️ Desinstalar Servicio"**
3. Confirme con **S**

## Soporte

Para soporte técnico, contacte a su proveedor o administrador del sistema.

---
Versión 1.0.0 - © 2024