package main

import (
	"context"
	"os"

	"github.com/int128/kubectl-external-forward/pkg/di"
)

var version = "v0.0.0"

func main() {
	os.Exit(di.NewCmd().Run(context.Background(), os.Args, version))
}
