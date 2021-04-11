package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/sync/errgroup"
)

func init() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
}

func main() {
	ctx := context.Background()
	eg, ctx := errgroup.WithContext(ctx)
	chInterrupt := make(chan struct{})
	eg.Go(func() error {
		defer close(chInterrupt)
		time.Sleep(5 * time.Second)
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 90 * time.Second
		return backoff.Retry(func() error { return openRequest(ctx) }, b)
	})
	eg.Go(func() error {
		if err := runExternalForward(chInterrupt); err != nil {
			log.Printf("runExternalForward: %s", err)
			return err
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}
}

func runExternalForward(chInterrupt <-chan struct{}) error {
	c := exec.Command("kubectl",
		"external-forward",
		"10000:www.example.com:80",
	)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return fmt.Errorf("could not start a process: %w", err)
	}
	log.Printf("started %s", c.String())
	defer func() {
		log.Printf("process state %s", c.ProcessState)
	}()
	<-chInterrupt
	if err := c.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("could not send SIGINT to the process: %w", err)
	}
	if err := c.Wait(); err != nil {
		return fmt.Errorf("wait error: %w", err)
	}
	return nil
}

func openRequest(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:10000/", nil)
	if err != nil {
		return fmt.Errorf("could not create a request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("received a response %s", resp.Status)
	return nil
}
