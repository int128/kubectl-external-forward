// Package cmd provides command line interface.
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/google/wire"
	"github.com/int128/kubectl-socat/pkg/externalforwarder"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

const (
	defaultImage = "ghcr.io/int128/kubectl-socat/mirror/alpine/socat:latest"
)

var Set = wire.NewSet(
	wire.Struct(new(Cmd), "*"),
	wire.Bind(new(Interface), new(*Cmd)),
)

type Interface interface {
	Run(ctx context.Context, osArgs []string, version string) int
}

// Cmd provides command line interface.
type Cmd struct {
	ExternalForwarder externalforwarder.Interface
}

// Run parses the arguments and executes the corresponding use-case.
func (cmd Cmd) Run(ctx context.Context, osArgs []string, version string) int {
	rootCmd := cmd.newRootCmd()
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.Version = version

	rootCmd.SetArgs(osArgs[1:])
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			klog.V(1).Infof("terminating: %s", err)
			return 0
		}
		klog.Infof("error: %s", err)
		klog.V(1).Infof("stacktrace: %+v", err)
		return 1
	}
	return 0
}

type rootCmdOptions struct {
	k8sOptions     *genericclioptions.ConfigFlags
	localPort      int
	remoteHostPort string
	image          string
}

func (o *rootCmdOptions) addFlags(f *pflag.FlagSet) {
	o.k8sOptions.AddFlags(f)
}

func (cmd Cmd) newRootCmd() *cobra.Command {
	var o rootCmdOptions
	o.k8sOptions = genericclioptions.NewConfigFlags(false)
	c := &cobra.Command{
		Use:     "kubectl socat [flags] [LOCAL_HOST:]LOCAL_PORT:REMOTE_HOST:REMOTE_PORT...",
		Short:   "TODO",
		Example: `kubectl socat 10000:db.staging:5432`,
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.runRootCmd(c.Context(), o, args)
		},
	}
	o.addFlags(c.Flags())
	c.Flags().IntVarP(&o.localPort, "local-port", "l", 0, "local port")
	c.Flags().StringVarP(&o.remoteHostPort, "remote-host", "r", "", "remote host:port")
	c.Flags().StringVarP(&o.image, "image", "", defaultImage, "Pod image")

	gf := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(gf)
	c.PersistentFlags().AddGoFlagSet(gf)
	return c
}

func (cmd Cmd) runRootCmd(ctx context.Context, o rootCmdOptions, args []string) error {
	tunnels, err := parseTunnelArgs(args)
	if err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	restConfig, err := o.k8sOptions.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("could not load the config: %w", err)
	}
	namespace, _, err := o.k8sOptions.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return fmt.Errorf("could not determine the namespace: %w", err)
	}
	return cmd.ExternalForwarder.Do(ctx, externalforwarder.Option{
		Config:    restConfig,
		Tunnels:   tunnels,
		Namespace: namespace,
		PodImage:  o.image,
	})
}

func parseTunnelArgs(args []string) ([]externalforwarder.Tunnel, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("you need to specify one or more arguments")
	}
	var tunnels []externalforwarder.Tunnel
	for _, arg := range args {
		s := strings.Split(arg, ":")
		lh := "127.0.0.1"
		if len(s) > 4 || len(s) < 3 {
			return nil, fmt.Errorf("invalid argument %s", arg)
		}
		if len(s) == 4 {
			if net.ParseIP(s[0]) == nil {
				return nil, fmt.Errorf("invalid local host: %s", s[0])
			}
			lh = s[0]
			s = s[1:]
		}
		l, err := strconv.Atoi(s[0])
		if err != nil {
			return nil, fmt.Errorf("invalid local port: %w", err)
		}
		r, err := strconv.Atoi(s[2])
		if err != nil {
			return nil, fmt.Errorf("invalid local port: %w", err)
		}
		tunnels = append(tunnels, externalforwarder.Tunnel{
			LocalHost:  lh,
			LocalPort:  l,
			RemoteHost: s[1],
			RemotePort: r,
		})
	}
	return tunnels, nil
}
