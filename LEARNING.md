# Learning & Achievements - Scale Daemon

## Project Overview
**Scale Daemon** is a specialized Windows Service designed to interface with industrial scales (specifically Rhino BAR 8RS) via serial port (RS232). It acts as a middleware that reads weight data in real-time and broadcasts it to web clients via a WebSocket server. The project includes a self-contained TUI (Text User Interface) installer that manages the service lifecycle (install, uninstall, start, stop) and embeds the service binary for easy distribution.

## Tech Stack and Key Technologies
- **Language:** Go (Golang) 1.24+
- **Platform:** Windows (System Services)
- **Communication Protocols:**
  - **Serial (RS232):** For communicating with hardware scales.
  - **WebSocket:** For real-time data broadcasting to clients.
- **User Interface:**
  - **TUI (Text User Interface):** For the installer and management tool.
  - **HTML/JS:** For the client-side visualization (embedded).
- **Build & Automation:** Taskfile (Task), Go Modules.

## Notable Libraries
- **[go.bug.st/serial](https://github.com/bugst/go-serial):** Used for robust cross-platform serial port communication. Solved the complexity of configuring baud rates, timeouts, and reading raw bytes from hardware.
- **[nhooyr.io/websocket](https://github.com/nhooyr/websocket):** A minimal and idiomatic WebSocket library. Used to implement the real-time broadcast server with context support.
- **[github.com/judwhite/go-svc](https://github.com/judwhite/go-svc):** Abstraction layer for Windows Services. Solved the problem of handling Windows Service Control Manager (SCM) signals (Start, Stop, Pause) cleanly in Go.
- **[github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea):** The Elm Architecture for Go terminal apps. Used to create the interactive, animated installer.
- **[github.com/charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss):** For styling the terminal UI (colors, borders, layouts).

## Major Achievements and Skills Demonstrated
- **System-Level Programming:**
  - Designed and implemented a **Windows Service** that runs in the background, handling OS signals and automatic recovery.
  - Implemented **Serial Port communication** with robust error handling (reconnection logic, timeouts, noise filtering).
- **Concurrent & Network Programming:**
  - Built a **concurrent WebSocket broadcaster** that handles multiple connected clients simultaneously without blocking the serial reading loop.
  - Implemented a **thread-safe configuration hot-swap** mechanism, allowing the service to change serial ports or target devices without restarting.
- **DevOps & Tooling:**
  - Created a **self-contained installer** by embedding the service binary into the installer executable using Go's `embed` package (or similar mechanism).
  - Configured a **multi-environment build system** (Prod vs. Test) using `Taskfile` and linker flags (`-ldflags`) to inject build metadata (version, date, environment).
- **User Experience (DX/UX):**
  - Developed a professional **Terminal User Interface (TUI)** for the installer, featuring animated spinners, progress bars, and a menu-driven workflow, significantly improving the deployment experience.
  - Implemented a **Simulation Mode** for development, allowing the service to generate fake weight data when no physical scale is connected.

## Skills Gained/Reinforced
- **Go Concurrency Patterns:** usage of `sync.Mutex`, `channels`, `context`, and `sync.WaitGroup` for graceful shutdowns.
- **Hardware Interfacing:** Understanding of RS232 communication, baud rates, and binary data parsing.
- **Windows API:** Interacting with the Windows Service Control Manager (SCM).
- **TUI Development:** Building modern CLI tools with the Bubbletea framework.
- **Architecture Design:** Decoupling hardware reading (Producer) from network broadcasting (Consumer).