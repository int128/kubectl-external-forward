package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/int128/kubectl-socat/pkg/di"
)

var version = "v0.0.0"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cancel the context on interrupted (ctrl+c)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		cancel()
	}()

	os.Exit(di.NewCmd().Run(ctx, os.Args, version))
}
