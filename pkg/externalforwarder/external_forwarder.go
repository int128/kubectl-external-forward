package externalforwarder

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/google/wire"
	"github.com/int128/kubectl-external-forward/pkg/portforwarder"
	"github.com/int128/kubectl-external-forward/pkg/tunnel"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var Set = wire.NewSet(
	wire.Struct(new(ExternalForwarder), "*"),
	wire.Bind(new(Interface), new(*ExternalForwarder)),
)

type Option struct {
	Config    *rest.Config
	Tunnels   []tunnel.Tunnel
	Namespace string
	PodImage  string
}

type Interface interface {
	Do(ctx context.Context, o Option) error
}

type ExternalForwarder struct {
	PortForwarder portforwarder.Interface
}

func (f ExternalForwarder) Do(ctx context.Context, o Option) error {
	clientset, err := kubernetes.NewForConfig(o.Config)
	if err != nil {
		return fmt.Errorf("could not create a client set: %w", err)
	}

	klog.Infof("creating a pod")
	pod, err := newPod(o)
	if err != nil {
		return fmt.Errorf("could not generate pod spec: %w", err)
	}
	pod, err = clientset.CoreV1().Pods(o.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create pod: %w", err)
	}
	klog.Infof("created pod %s/%s", pod.Namespace, pod.Name)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	var eg errgroup.Group
	eg.Go(func() error {
		<-ctx.Done()

		// clean up the pod
		ctx := context.Background()
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
		klog.Infof("deleting pod %s/%s...", pod.Namespace, pod.Name)
		if err := deletePodWithRetry(ctx, clientset, pod.Namespace, pod.Name, 30*time.Second); err != nil {
			return fmt.Errorf("you need to delete pod %s/%s manually: %w", pod.Namespace, pod.Name, err)
		}
		klog.Infof("deleted pod %s/%s", pod.Namespace, pod.Name)
		return nil
	})

	eg.Go(func() error {
		if err := waitForPodRunning(ctx, clientset, pod.Namespace, pod.Name, 30*time.Second); err != nil {
			return fmt.Errorf("pod is not running: %w", err)
		}

		for _, container := range pod.Spec.Containers {
			containerName := container.Name
			eg.Go(func() error {
				return tailPodLogs(ctx, clientset, pod.Namespace, pod.Name, containerName)
			})
		}

		for _, t := range o.Tunnels {
			f.startPortForwarder(ctx, &eg, o.Config, t, pod)
		}
		return nil
	})
	return eg.Wait()
}

func (f ExternalForwarder) startPortForwarder(ctx context.Context, eg *errgroup.Group, restConfig *rest.Config, tunnel tunnel.Tunnel, pod *corev1.Pod) {
	stopChan := make(chan struct{})
	eg.Go(func() error {
		<-ctx.Done()
		close(stopChan)
		return nil
	})
	eg.Go(func() error {
		klog.Infof("starting port-forwarder from %d to %s/%s:%d", tunnel.LocalPort, pod.Namespace, pod.Name, tunnel.LocalPort)
		po := portforwarder.Option{
			Config:              restConfig,
			SourceHost:          tunnel.LocalHost,
			SourcePort:          tunnel.LocalPort,
			TargetNamespace:     pod.Namespace,
			TargetPodName:       pod.Name,
			TargetContainerPort: tunnel.LocalPort,
		}
		if err := f.PortForwarder.Run(po, nil, stopChan); err != nil {
			return fmt.Errorf("could not start port-forwarder")
		}
		klog.Info("stopped port-forwarder")
		return nil
	})
}
