package agentotel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestForceFlushAndShutdownOrderAndErrors(t *testing.T) {
	forceErr := errors.New("force metrics")
	shutdownErr := errors.New("shutdown metrics")
	var calls []string
	cleanup := newShutdown(
		[]func(context.Context) error{
			recordCall(&calls, "force logs", nil),
			recordCall(&calls, "force metrics", forceErr),
			recordCall(&calls, "force traces", nil),
		},
		[]func(context.Context) error{
			recordCall(&calls, "shutdown logs", nil),
			recordCall(&calls, "shutdown metrics", shutdownErr),
			recordCall(&calls, "shutdown traces", nil),
		},
	)

	require.ErrorIs(t, cleanup.ForceFlush(context.Background()), forceErr)
	require.ErrorIs(t, cleanup.Shutdown(context.Background()), shutdownErr)
	require.Equal(t, []string{
		"force logs",
		"force metrics",
		"force traces",
		"shutdown logs",
		"shutdown metrics",
		"shutdown traces",
	}, calls)
}

func TestShutdownIsIdempotentAndReturnsSameError(t *testing.T) {
	shutdownErr := errors.New("shutdown failed")
	var calls []string
	cleanup := newShutdown(nil, []func(context.Context) error{
		recordCall(&calls, "shutdown logs", shutdownErr),
		recordCall(&calls, "shutdown metrics", nil),
	})

	require.ErrorIs(t, cleanup.Shutdown(context.Background()), shutdownErr)
	require.ErrorIs(t, cleanup.Shutdown(context.Background()), shutdownErr)
	require.Equal(t, []string{"shutdown logs", "shutdown metrics"}, calls)
}

func TestCallAllContinuesAfterError(t *testing.T) {
	firstErr := errors.New("first")
	secondErr := errors.New("second")
	var calls []string

	err := callAll(context.Background(), []func(context.Context) error{
		recordCall(&calls, "first", firstErr),
		nil,
		recordCall(&calls, "second", secondErr),
		recordCall(&calls, "third", nil),
	})

	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.Equal(t, []string{"first", "second", "third"}, calls)
}

func TestCallAllPropagatesContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	var calls []string
	err := callAll(ctx, []func(context.Context) error{
		func(ctx context.Context) error {
			calls = append(calls, "logs")
			return ctx.Err()
		},
		recordCall(&calls, "metrics", nil),
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, []string{"logs", "metrics"}, calls)
}

func recordCall(calls *[]string, name string, err error) func(context.Context) error {
	return func(context.Context) error {
		*calls = append(*calls, name)
		return err
	}
}
