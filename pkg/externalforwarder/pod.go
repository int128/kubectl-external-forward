package externalforwarder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff/v4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func waitForPodRunning(ctx context.Context, c *kubernetes.Clientset, namespace, name string, timeout time.Duration) error {
	checkIfRunning := func() error {
		pod, err := c.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return backoff.Permanent(err)
		}
		if pod.Status.Phase != corev1.PodRunning {
			return fmt.Errorf("pod %s/%s is still %s", pod.Namespace, pod.Name, pod.Status.Phase)
		}
		return nil
	}
	notify := func(err error, d time.Duration) {
		klog.Info(err)
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout
	if err := backoff.RetryNotify(checkIfRunning, backoff.WithContext(b, ctx), notify); err != nil {
		return err
	}
	return nil
}

func tailPodLogs(ctx context.Context, c *kubernetes.Clientset, namespace, name string) error {
	stream, err := c.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{Follow: true}).Stream(ctx)
	if err != nil {
		return fmt.Errorf("could not get logs from pod: %w", err)
	}
	defer stream.Close()
	for {
		r := bufio.NewReader(stream)
		l, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}
		klog.Infof("%s/%s: %s", namespace, name, l)
	}
}

func deletePodWithRetry(ctx context.Context, c *kubernetes.Clientset, namespace, name string, timeout time.Duration) error {
	attempt := func() error {
		err := c.CoreV1().Pods(namespace).Delete(ctx, name, *metav1.NewDeleteOptions(0))
		if err != nil {
			return fmt.Errorf("could not delete pod: %w", err)
		}
		return nil
	}
	notify := func(err error, d time.Duration) {
		klog.Info(err)
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout
	if err := backoff.RetryNotify(attempt, backoff.WithContext(b, ctx), notify); err != nil {
		return err
	}
	return nil
}
