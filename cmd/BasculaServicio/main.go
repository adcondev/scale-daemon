package main

import (
	"log"
	"syscall"

	"github.com/judwhite/go-svc"

	"github.com/adcondev/scale-daemon/internal/daemon"
)

// Build variables (injected via ldflags)
var (
	BuildEnvironment = "local"
	ServiceName      = ""        // Injected for logging path (e.g., "R2k_BasculaServicio_Local")
	BuildDate        = "unknown"
	BuildTime        = "unknown"
)

func main() {
	// Create and run service
	service := daemon.New(BuildEnvironment, ServiceName, BuildDate, BuildTime)

	if err := svc.Run(service, syscall.SIGINT, syscall.SIGTERM); err != nil {
		log.Fatal(err)
	}
}
