package externalforwarder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/int128/kubectl-external-forward/pkg/envoy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func newPod(o Option) (*corev1.Pod, error) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubectl-external-forward-",
			Annotations: map[string]string{
				// do not prevent scale-in of cluster autoscaler
				"cluster-autoscaler.kubernetes.io/safe-to-evict": "true",
			},
		},
		Spec: corev1.PodSpec{},
	}

	envoyConfig, err := envoy.NewConfig(o.Tunnels)
	if err != nil {
		return nil, fmt.Errorf("could not generate envoy config: %w", err)
	}

	pod.Spec.Containers = []corev1.Container{
		{
			Name:  "envoy",
			Image: o.PodImage,
			Args: []string{
				"--config-yaml",
				envoyConfig,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("10Mi"),
				},
			},
		},
	}
	return &pod, nil
}

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

func tailPodLogs(ctx context.Context, c *kubernetes.Clientset, namespace, name, containerName string) error {
	opts := corev1.PodLogOptions{
		Follow:    true,
		Container: containerName,
	}
	stream, err := c.CoreV1().Pods(namespace).GetLogs(name, &opts).Stream(ctx)
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
		klog.Infof("%s/%s/%s: %s", namespace, name, containerName, l)
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
