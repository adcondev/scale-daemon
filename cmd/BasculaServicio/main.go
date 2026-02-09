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
	BuildDate        = "unknown"
	BuildTime        = "unknown"
	LogServiceName   = "" // Used for log directory naming, not Windows SCM service name
)

func main() {
	// Create and run service
	service := daemon.New(BuildEnvironment, BuildDate, BuildTime, LogServiceName)

	if err := svc.Run(service, syscall.SIGINT, syscall.SIGTERM); err != nil {
		log.Fatal(err)
	}
}
