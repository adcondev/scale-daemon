# Learning & Achievements - Scale Daemon

## Project Overview

**Scale Daemon** is a production-grade Windows Service written in Go that bridges industrial weighing scales (
RS232/Serial) with modern web applications. It reads weight data in real-time from serial-connected hardware and
broadcasts it to multiple clients simultaneously via a low-latency WebSocket server, serving as persistent middleware
for retail and logistics environments.

## Tech Stack & Infrastructure

- **Language:** Go (Golang) 1.24+
- **Platform:** Windows (System Services via SCM)
- **Communication Protocols:**
    - **Serial (RS232):** 9600 baud, hardware scale communication with configurable timeouts
    - **WebSocket:** Real-time bidirectional streaming (hybrid JSON/string protocol)
    - **HTTP:** REST endpoints for health checks, diagnostics, and dashboard serving
- **Frontend:** Embedded HTML/CSS/JS dashboard served via Go's `embed` package
- **Authentication:** bcrypt password hashing (via `golang.org/x/crypto`), session-based auth with HttpOnly cookies
- **Build & Automation:** Taskfile (Task v3), Go Modules, `ldflags` injection for secrets and metadata
- **CI/CD:** GitHub Actions (test, lint, build, benchmark, PR automation, PR status dashboard)
- **Security Scanning:** GitHub CodeQL (weekly + on PR), golangci-lint with `gosec` enabled
- **Code Coverage:** Codecov integration with race detection
- **API Specification:** JSON Schema (Draft-07) for WebSocket protocol

## Notable Libraries

- **[go.bug.st/serial](https://github.com/bugst/go-serial):** Cross-platform serial port communication. Solved
  configuring baud rates, read timeouts, and handling raw bytes from industrial hardware.
- **[github.com/coder/websocket](https://github.com/coder/websocket):** Minimal, idiomatic WebSocket library with
  `context.Context` support. Used for real-time broadcast server and JSON message handling via `wsjson`.
- **[github.com/judwhite/go-svc](https://github.com/judwhite/go-svc):** Abstraction for Windows Service Control
  Manager (SCM). Handles OS signals (SIGINT, SIGTERM) and service lifecycle (Init, Start, Stop) cleanly in Go.
- **[golang.org/x/crypto/bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt):** Industry-standard password hashing.
  Used to validate dashboard login credentials without storing plaintext passwords in the binary.

## CV-Ready Achievements

- **Architected** a concurrent, event-driven Windows Service in Go that interfaces with RS232 industrial scales, using a
  Producer–Consumer pattern with goroutines and channels to decouple hardware I/O from WebSocket fan-out broadcasting to
  multiple clients simultaneously.
- **Engineered** a layered security system featuring bcrypt password authentication, cryptographically random session
  tokens (HttpOnly/SameSite cookies), per-IP brute-force lockout (5 attempts → 5-minute ban), and per-client WebSocket
  rate limiting (15 config changes/min) — all with comprehensive `[AUDIT]` logging.
- **Developed** a real-time embedded web dashboard using Go's `embed` package and HTML templates, with server-side token
  injection to solve the "static file authentication paradox" for WebSocket config authorization.
- **Implemented** a hybrid WebSocket API protocol (JSON objects for control messages, raw JSON strings for weight
  streaming) validated by a JSON Schema specification, minimizing overhead and latency for high-frequency weight data
  broadcasting.
- **Optimized** service reliability with automatic serial port reconnection, configurable log rotation (5 MB threshold
  with last-1000-line preservation), verbose/non-verbose log filtering, and graceful multi-component shutdown with
  context cancellation and `sync.WaitGroup` coordination.
- **Built** a comprehensive CI/CD pipeline using GitHub Actions with 5 workflows: automated testing with race detection,
  Codecov coverage reporting, `golangci-lint` with 15+ linters (including `gosec`), GitHub CodeQL security analysis (
  weekly + on PR), performance benchmarking with base/PR comparison, semantic PR title validation, auto-labeling, and a
  weekly PR status dashboard.
- **Designed** a hot-swappable runtime configuration system allowing serial port, scale brand, and test mode changes via
  authenticated WebSocket messages without service restarts, using `sync.RWMutex` for thread-safe config snapshots.

## Skills Demonstrated

Go Concurrent Programming, Windows Service Development, Serial Port Communication (RS232), WebSocket Real-Time
Streaming, REST API Design, Authentication & Session Management, bcrypt Password Hashing, Brute-Force Protection, Rate
Limiting, Embedded File Systems (go:embed), CI/CD Pipeline Design (GitHub Actions), Static Analysis & Security
Scanning (golangci-lint, CodeQL, gosec), Performance Benchmarking, JSON Schema API Specification, Log Rotation &
Filtering, Graceful Shutdown Patterns, Producer–Consumer Architecture, Build-Time Secret Injection (ldflags)