package externalforwarder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff/v4"
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

	klog.Infof("creating a socat pod")
	socatPod, err := clientset.CoreV1().Pods(o.Namespace).Create(ctx, &corev1.Pod{
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
	klog.Infof("created socat pod: %s/%s", socatPod.Namespace, socatPod.Name)

	stopChan := make(chan struct{})
	var eg errgroup.Group
	eg.Go(func() error {
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 30 * time.Second
		if err := backoff.Retry(func() error {
			socatPod, err := clientset.CoreV1().Pods(o.Namespace).Get(ctx, socatPod.Name, metav1.GetOptions{})
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
			socatLogStream, err := clientset.CoreV1().Pods(o.Namespace).GetLogs(socatPod.Name, &corev1.PodLogOptions{Follow: true}).Stream(ctx)
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
