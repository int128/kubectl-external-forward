package externalforwarder

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/google/wire"
	"github.com/int128/kubectl-socat/pkg/portforwarder"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var Set = wire.NewSet(
	wire.Struct(new(ExternalForwarder), "*"),
	wire.Bind(new(Interface), new(*ExternalForwarder)),
)

type Option struct {
	Config         *rest.Config
	Namespace      string
	LocalPort      int
	RemoteHostPort string
	PodImage       string
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

	klog.Infof("creating a socat pod with image %s", o.PodImage)
	socatPod, err := clientset.CoreV1().Pods(o.Namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "socat-",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "socat",
					Image: o.PodImage,
					Args: []string{
						"-dd",
						fmt.Sprintf("tcp-listen:%d,fork", o.LocalPort),
						fmt.Sprintf("tcp-connect:%s", o.RemoteHostPort),
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create socat pod: %w", err)
	}
	klog.Infof("created pod %s/%s", socatPod.Namespace, socatPod.Name)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	var eg errgroup.Group
	eg.Go(func() error {
		<-ctx.Done()

		// clean up the pod
		ctx := context.Background()
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
		klog.Infof("deleting pod %s/%s...", socatPod.Namespace, socatPod.Name)
		if err := deletePodWithRetry(ctx, clientset, socatPod.Namespace, socatPod.Name, 30*time.Second); err != nil {
			return fmt.Errorf("you need to delete pod %s/%s manually: %w", socatPod.Namespace, socatPod.Name, err)
		}
		klog.Infof("deleted pod %s/%s", socatPod.Namespace, socatPod.Name)
		return nil
	})

	eg.Go(func() error {
		if err := waitForPodRunning(ctx, clientset, socatPod.Namespace, socatPod.Name, 30*time.Second); err != nil {
			return fmt.Errorf("socat pod is not running: %w", err)
		}

		eg.Go(func() error {
			if err := tailPodLogs(ctx, clientset, socatPod.Namespace, socatPod.Name); err != nil {
				return fmt.Errorf("could not tail logs: %w", err)
			}
			return nil
		})

		stopChan := make(chan struct{})
		eg.Go(func() error {
			<-ctx.Done()
			close(stopChan)
			return nil
		})
		eg.Go(func() error {
			klog.Infof("starting port-forwarder from %d to %s/%s:%d", o.LocalPort, socatPod.Namespace, socatPod.Name, o.LocalPort)
			po := portforwarder.Option{
				Config:              o.Config,
				SourcePort:          o.LocalPort,
				TargetNamespace:     socatPod.Namespace,
				TargetPodName:       socatPod.Name,
				TargetContainerPort: o.LocalPort,
			}
			if err := f.PortForwarder.Run(po, nil, stopChan); err != nil {
				return fmt.Errorf("could not start port-forwarder")
			}
			klog.Info("stopped port-forwarder")
			return nil
		})
		return nil
	})
	return eg.Wait()
}
