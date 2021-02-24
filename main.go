package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type options struct {
	localPort      string
	remoteHostPort string
	podName        string
	image          string
}

func newSocatCmd(ctx context.Context, o options) *exec.Cmd {
	c := exec.CommandContext(ctx,
		"kubectl", "run", o.podName, "--rm", "-it", "--image", o.image,
		"--",
		"-dd",
		fmt.Sprintf("tcp-listen:%s,fork", o.localPort),
		fmt.Sprintf("tcp-connect:%s", o.remoteHostPort),
	)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func newWaitForPodCmd(ctx context.Context, o options) *exec.Cmd {
	c := exec.CommandContext(ctx, "kubectl", "wait", "--for=condition=Ready", "pod/"+o.podName)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func newPortForwardCmd(ctx context.Context, o options) *exec.Cmd {
	c := exec.CommandContext(ctx, "kubectl", "port-forward", o.podName, o.localPort)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func runSocatAndPortForward(ctx context.Context, o options) error {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer cancel()

		socatCmd := newSocatCmd(ctx, o)
		log.Printf("socat: %s", socatCmd)
		if err := socatCmd.Start(); err != nil {
			return fmt.Errorf("socat error: %w", err)
		}
		err := retryWithContext(ctx, 10, time.Second, func() error {
			waitForPodCmd := newWaitForPodCmd(ctx, o)
			log.Printf("wait-for-pod: %s", waitForPodCmd)
			return waitForPodCmd.Run()
		})
		if err != nil {
			return fmt.Errorf("wait-for-pod error: %w", err)
		}

		eg.Go(func() error {
			defer cancel()

			portForwardCmd := newPortForwardCmd(ctx, o)
			log.Printf("port-forward: %s", portForwardCmd)
			if err := portForwardCmd.Run(); err != nil {
				// TODO: prevent error when socat successfully exited
				return fmt.Errorf("port-forward error: %w", err)
			}
			log.Printf("port-forward: exit")
			return nil
		})

		if err := socatCmd.Wait(); err != nil {
			// TODO: ensure the pod is deleted
			return fmt.Errorf("socat error: %w", err)
		}
		log.Printf("socat: exit")
		return nil
	})
	return eg.Wait()
}

func randomPodSuffix() string {
	b := make([]byte, 15)
	_, _ = rand.Read(b)
	s := base32.StdEncoding.EncodeToString(b)
	s = strings.ToLower(s)
	return s
}

func run(ctx context.Context) error {
	var o options
	flag.StringVar(&o.localPort, "l", "", "local port of this machine")
	flag.StringVar(&o.remoteHostPort, "r", "", "remote host:port to connect")
	flag.StringVar(&o.podName, "pod", "socat-"+randomPodSuffix(), "pod name to run socat")
	flag.StringVar(&o.image, "image", "alpine/socat", "container image of socat")
	flag.Parse()
	if o.localPort == "" {
		return fmt.Errorf("you need to set the local port by -l flag")
	}
	if o.remoteHostPort == "" {
		return fmt.Errorf("you need to set the remote host by -r flag")
	}
	return runSocatAndPortForward(ctx, o)
}

func main() {
	log.SetFlags(log.Lmicroseconds)
	err := run(context.Background())
	if err != nil {
		log.Fatalf("error: %s", err)
	}
}
