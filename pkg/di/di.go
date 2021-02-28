//+build wireinject

// Package di provides dependency injection.
package di

//go:generate wire

import (
	"github.com/google/wire"
	"github.com/int128/kubectl-socat/pkg/cmd"
	"github.com/int128/kubectl-socat/pkg/portforwarder"
)

func NewCmd() cmd.Interface {
	wire.Build(
		cmd.Set,
		portforwarder.Set,
	)
	return nil
}
