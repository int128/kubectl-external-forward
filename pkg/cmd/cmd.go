// Package cmd provides command line interface.
package cmd

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/wire"
	"github.com/int128/kubectl-socat/pkg/portforwarder"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	PortForwarder portforwarder.Interface
}

// Run parses the arguments and executes the corresponding use-case.
func (cmd *Cmd) Run(ctx context.Context, osArgs []string, version string) int {
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
}

func (o *rootCmdOptions) addFlags(f *pflag.FlagSet) {
	o.k8sOptions.AddFlags(f)
}

func (cmd *Cmd) newRootCmd() *cobra.Command {
	var o rootCmdOptions
	o.k8sOptions = genericclioptions.NewConfigFlags(false)
	c := &cobra.Command{
		Use:     "kubectl socat",
		Short:   "TODO",
		Example: "TODO",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			return cmd.runRootCmd(c.Context(), o, args)
		},
	}
	o.addFlags(c.Flags())
	c.Flags().IntVarP(&o.localPort, "local-port", "l", 0, "local port")
	c.Flags().StringVarP(&o.remoteHostPort, "remote-host", "r", "", "remote host:port")

	gf := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(gf)
	c.PersistentFlags().AddGoFlagSet(gf)
	return c
}

func (cmd *Cmd) runRootCmd(ctx context.Context, o rootCmdOptions, _ []string) error {
	restConfig, err := o.k8sOptions.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("could not load the config: %w", err)
	}
	namespace, _, err := o.k8sOptions.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return fmt.Errorf("could not determine the namespace: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("could not create a client set: %w", err)
	}

	//TODO: extract use-case
	klog.Infof("creating a socat pod")
	socatPod, err := clientset.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "socat-",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "socat",
					Image: "alpine/socat:latest",
					Args: []string{
						"-dd",
						fmt.Sprintf("tcp-listen:%d,fork", o.localPort),
						fmt.Sprintf("tcp-connect:%s", o.remoteHostPort),
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create socat pod: %w", err)
	}
	klog.Infof("created socat pod: %s/%s", socatPod.Namespace, socatPod.Name)

	stopChan := make(chan struct{})
	var eg errgroup.Group
	eg.Go(func() error {
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 30 * time.Second
		if err := backoff.Retry(func() error {
			socatPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, socatPod.Name, metav1.GetOptions{})
			if err != nil {
				klog.Infof("waiting for socat pod: %s", err)
				return err
			}
			if socatPod.Status.Phase != corev1.PodRunning {
				klog.Infof("waiting for socat pod: %s: %s", socatPod.Status.Phase, socatPod.Status.Message)
				return fmt.Errorf("pod %s/%s is not running", socatPod.Namespace, socatPod.Name)
			}
			return nil
		}, backoff.WithContext(b, ctx)); err != nil {
			return fmt.Errorf("could not run socat pod: %w", err)
		}

		eg.Go(func() error {
			socatLogStream, err := clientset.CoreV1().Pods(namespace).GetLogs(socatPod.Name, &corev1.PodLogOptions{Follow: true}).Stream(ctx)
			if err != nil {
				return fmt.Errorf("could not get logs from socat pod: %w", err)
			}
			defer socatLogStream.Close()
			for {
				r := bufio.NewReader(socatLogStream)
				l, _, err := r.ReadLine()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return fmt.Errorf("could not read log from socat pod: %w", err)
				}
				klog.Infof("socat: %s", l)
			}
		})
		eg.Go(func() error {
			klog.Infof("starting port-forwarder from %d to %s/%s:%d", o.localPort, socatPod.Namespace, socatPod.Name, o.localPort)
			po := portforwarder.Option{
				Config:              restConfig,
				SourcePort:          o.localPort,
				TargetNamespace:     socatPod.Namespace,
				TargetPodName:       socatPod.Name,
				TargetContainerPort: o.localPort,
			}
			if err := cmd.PortForwarder.Run(po, nil, stopChan); err != nil {
				return fmt.Errorf("could not start port-forwarder")
			}
			klog.Info("stopped port-forwarder")
			return nil
		})
		return nil
	})
	eg.Go(func() error {
		<-ctx.Done()
		close(stopChan)
		ctx := context.Background()
		klog.Infof("deleting socat pod %s/%s", socatPod.Namespace, socatPod.Name)
		err := clientset.CoreV1().Pods(socatPod.Namespace).Delete(ctx, socatPod.Name, *metav1.NewDeleteOptions(0))
		if err != nil {
			return fmt.Errorf("could not delete socat pod: %w", err)
		}
		klog.Infof("deleted socat pod %s/%s", socatPod.Namespace, socatPod.Name)
		return nil
	})
	return eg.Wait()
}
