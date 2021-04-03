//+build wireinject

// Package di provides dependency injection.
package di

//go:generate wire

import (
	"github.com/google/wire"
	"github.com/int128/kubectl-external-forward/pkg/cmd"
	"github.com/int128/kubectl-external-forward/pkg/externalforwarder"
	"github.com/int128/kubectl-external-forward/pkg/portforwarder"
)

func NewCmd() cmd.Interface {
	wire.Build(
		cmd.Set,
		portforwarder.Set,
		externalforwarder.Set,
	)
	return nil
}
