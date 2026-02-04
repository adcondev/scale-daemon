package main

import (
	"log"
	"math/rand"
	"syscall"
	"time"

	"github.com/judwhite/go-svc"

	"github.com/adcondev/scale-daemon/internal/daemon"
)

// Build variables (injected via ldflags)
var (
	BuildEnvironment = "test"
	BuildDate        = "unknown"
	BuildTime        = "unknown"
)

func main() {
	// Seed random for test mode weight simulation
	rand.Seed(time.Now().UnixNano())

	// Create and run service
	service := daemon.New(BuildEnvironment, BuildDate, BuildTime)

	if err := svc.Run(service, syscall.SIGINT, syscall.SIGTERM); err != nil {
		log.Fatal(err)
	}
}
