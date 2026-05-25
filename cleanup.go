package agentotel

import (
	"context"
	"errors"
	"sync"
)

// Shutdown coordinates ForceFlush and Shutdown calls for initialized providers.
type Shutdown struct {
	forceFlush []func(context.Context) error
	shutdown   []func(context.Context) error

	mu          sync.Mutex
	shutdownErr error
	shutDown    bool
}

func newShutdown(forceFlush, shutdown []func(context.Context) error) *Shutdown {
	return &Shutdown{
		forceFlush: forceFlush,
		shutdown:   shutdown,
	}
}

// ForceFlush flushes logs, metrics, and traces in that order.
func (s *Shutdown) ForceFlush(ctx context.Context) error {
	if s == nil {
		return nil
	}
	return callAll(ctx, s.forceFlush)
}

// Shutdown stops logs, metrics, and traces in that order. It is safe to call more than once.
func (s *Shutdown) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	if s.shutDown {
		err := s.shutdownErr
		s.mu.Unlock()
		return err
	}
	s.shutDown = true
	s.mu.Unlock()

	err := callAll(ctx, s.shutdown)

	s.mu.Lock()
	s.shutdownErr = err
	s.mu.Unlock()

	return err
}

func callAll(ctx context.Context, calls []func(context.Context) error) error {
	var joined error
	for _, call := range calls {
		if call == nil {
			continue
		}
		joined = errors.Join(joined, call(ctx))
	}
	return joined
}
