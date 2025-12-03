# Project Technical Summary & Learning

## Project Overview

This project is a Go application designed to run as a Windows service. It reads data from a serial port (specifically from a weighing scale) and broadcasts it to web clients via WebSockets. The project also includes a Terminal User Interface (TUI) installer built with the Bubble Tea library, which embeds the main service executable within its binary for a seamless installation experience.

## Tech Stack and Key Technologies

*   **Language:** Go
*   **Platform:** Windows (designed to run as a Windows Service)
*   **Build Automation:** Taskfile (`Taskfile.yml`)
*   **Frontend:** HTML, CSS, JavaScript (for the client-side WebSocket consumer)
*   **Dependency Management:** Go Modules

## Notable Libraries

*   **`go.bug.st/serial`**: Used for serial port communication, which is the core of reading data from the weighing scale.
*   **`nhooyr.io/websocket`**: A high-performance WebSocket library for Go, used to create the WebSocket server that broadcasts the scale's data to clients.
*   **`github.com/judwhite/go-svc`**: A library for creating and managing Windows services in Go. This is essential for running the application in the background on Windows.
*   **`github.com/charmbracelet/bubbletea`**: A powerful framework for building terminal-based user interfaces (TUIs). This is used to create the interactive installer for the service.
*   **`github.com/charmbracelet/lipgloss`**: Used for styling the TUI, providing a polished and professional look and feel to the installer.
*   **`//go:embed`**: A Go directive used to embed the service executable directly into the installer, creating a single, self-contained distributable.

## Major Achievements and Skills Demonstrated

*   **Designed and implemented a Windows service in Go:** Created a robust, long-running application that can be managed by the Windows Service Control Manager.
*   **Developed a real-time data broadcasting system:** Built a WebSocket server to broadcast data from a serial port to multiple web clients in real-time.
*   **Created an interactive TUI installer:** Developed a user-friendly installer with a terminal-based interface using the Bubble Tea framework.
*   **Implemented a self-contained application bundle:** Utilized Go's `embed` directive to package the service executable within the installer, simplifying distribution and installation.
*   **Managed build automation with Taskfile:** Created a `Taskfile.yml` to automate the build process for different environments (production and test).
*   **Implemented environment-specific configurations:** Designed the application to be built with different configurations for production and testing environments.

## Skills Gained/Reinforced

*   **Concurrent Programming:** Utilized goroutines and channels to handle concurrent WebSocket connections and serial port reading.
*   **Systems Programming:** Gained experience in creating and managing Windows services.
*   **TUI Development:** Learned how to build interactive and user-friendly terminal applications with Bubble Tea.
*   **API Design (WebSocket):** Designed a simple WebSocket-based protocol for real-time communication between the service and web clients.
*   **Build Automation:** Gained proficiency in using Taskfile for automating build processes.
*   **Go Language Proficiency:** Deepened understanding of Go's features, including concurrency, modules, and the `embed` directive.

## Advanced Technical Concepts Implemented

*   **Graceful Shutdown & Context Management:**
    *   Implemented `context.Context` propagation to ensure clean cancellation of goroutines (WebSocket writers, config listeners) when the service stops.
    *   Integrated with `go-svc` to handle Windows Service Control Manager (SCM) signals (Stop/Shutdown) and gracefully terminate the application.

*   **Event-Driven Architecture (TUI):**
    *   Adopted The Elm Architecture (Model-View-Update) pattern via the Bubble Tea framework for the installer.
    *   Managed complex state transitions (Menu -> Processing -> Result -> Confirm) in a purely functional way.

*   **Robust Error Handling & Reconnection:**
    *   Designed a self-healing serial connection loop that automatically attempts to reconnect upon disconnection or timeout, preventing service failure in unstable hardware environments.
    *   Implemented exponential backoff (or fixed delay) strategies to avoid tight loops during hardware failures.

*   **JSON-RPC Style Communication:**
    *   Established a bidirectional JSON-based protocol over WebSockets.
    *   Clients can send configuration commands (`{"tipo": "config", ...}`) which dynamically update the running service's state (serial port, brand) without a restart.