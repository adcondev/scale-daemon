// Package main implements the main entry point for the scale daemon application.
package main

import (
	"log"
	"syscall"

	"github.com/judwhite/go-svc"

	"github.com/adcondev/scale-daemon/internal/config"
	"github.com/adcondev/scale-daemon/internal/daemon"
)

func main() {
	// Create and run service
	service := daemon.New(config.BuildEnvironment, config.BuildDate, config.BuildTime)
	if err := svc.Run(service, syscall.SIGINT, syscall.SIGTERM); err != nil {
		log.Fatal(err)
	}
}
