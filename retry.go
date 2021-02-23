package main

import (
	"context"
	"fmt"
	"time"
)

func retryWithContext(ctx context.Context, maxRetries int, interval time.Duration, f func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err := waitWithContext(ctx, interval); err != nil {
			return err
		}
		err = f()
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("retry over: %w", err)
}

func waitWithContext(ctx context.Context, d time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timeoutCtx.Done():
		return nil
	}
}
