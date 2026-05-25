//go:build integration

package lotelhelper

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentotel "github.com/mattsp1290/agent-otel"
)

func TestStart_SkipsWhenCLIMissing(t *testing.T) {
	resetPkgState(t)
	missing := filepath.Join(t.TempDir(), "definitely-not-lotel-cli")
	t.Setenv(envLotelCLI, missing)

	rec := &recordingTB{TB: t}
	done := make(chan struct{})
	var h *Helper
	go func() {
		defer close(done)
		h = Start(rec)
	}()
	<-done

	if h != nil {
		t.Fatalf("Start returned non-nil helper despite missing CLI")
	}
	if !rec.skipped {
		t.Fatalf("Start did not call Skip; got fatals=%v errors=%v", rec.fatals, rec.errors)
	}
	if !strings.Contains(rec.skipMsg, missing) {
		t.Errorf("Skip message %q did not mention %q", rec.skipMsg, missing)
	}
}

func TestStart_HappyPathQueriesEmittedSpan(t *testing.T) {
	resetPkgState(t)
	h := Start(t)
	if h == nil {
		return
	}

	if got := h.Endpoint(); got != OTLPGRPCEndpoint {
		t.Errorf("Endpoint() = %q, want %q", got, OTLPGRPCEndpoint)
	}

	service := fmt.Sprintf("agent-otel-lotelhelper-test-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	providers, shutdown, err := agentotel.Init(ctx, agentotel.Options{
		Enabled:           true,
		ServiceName:       service,
		SkipGlobalInstall: true,
		DialTimeout:       time.Second,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
		TraceExporter: agentotel.ExporterConfig{
			Endpoint: h.Endpoint(),
			Protocol: agentotel.ProtocolGRPC,
			Insecure: true,
		},
		MetricExporter: agentotel.ExporterConfig{
			Endpoint: h.Endpoint(),
			Protocol: agentotel.ProtocolGRPC,
			Insecure: true,
		},
		LogExporter: agentotel.ExporterConfig{
			Endpoint: h.Endpoint(),
			Protocol: agentotel.ProtocolGRPC,
			Insecure: true,
		},
	})
	if err != nil {
		t.Fatalf("agentotel.Init: %v", err)
	}

	_, span := providers.Tracer.Start(ctx, "lotelhelper-smoke-span")
	span.End()

	if err := shutdown.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}
	if err := shutdown.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	records := h.WaitForTraces(t, service, 1, 15*time.Second)
	if len(records) == 0 {
		t.Fatalf("WaitForTraces(%q) returned no records", service)
	}
}

func resetPkgState(t testing.TB) {
	t.Helper()
	pkgMu.Lock()
	if pkgRefs != 0 {
		refs := pkgRefs
		pkgMu.Unlock()
		t.Fatalf("lotelhelper: resetPkgState called with %d Helper(s) still in flight", refs)
	}
	pkgRefs = 0
	pkgStarted = false
	pkgCLI = ""
	pkgMu.Unlock()
}

type recordingTB struct {
	testing.TB
	skipped bool
	skipMsg string
	fatals  []string
	errors  []string
}

func (r *recordingTB) Skip(args ...any) {
	r.skipped = true
	r.skipMsg = fmt.Sprint(args...)
	runtime.Goexit()
}

func (r *recordingTB) Skipf(format string, args ...any) {
	r.skipped = true
	r.skipMsg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func (r *recordingTB) SkipNow() {
	r.skipped = true
	runtime.Goexit()
}

func (r *recordingTB) Fatal(args ...any) {
	r.fatals = append(r.fatals, fmt.Sprint(args...))
	runtime.Goexit()
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
	runtime.Goexit()
}

func (r *recordingTB) Error(args ...any) {
	r.errors = append(r.errors, fmt.Sprint(args...))
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}
