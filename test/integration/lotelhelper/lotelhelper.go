//go:build integration

// Package lotelhelper wraps lotel-cli for integration tests that assert on
// telemetry exported through a real local lotel collector.
package lotelhelper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	OTLPGRPCEndpoint = "localhost:4317"
	OTLPHTTPEndpoint = "localhost:4318"
	HealthEndpoint   = "localhost:13133"

	envLotelCLI    = "LOTEL_CLI"
	fallbackCLIRel = "git/lotel/target/release/lotel-cli"

	startTimeout  = 30 * time.Second
	stopTimeout   = 10 * time.Second
	healthTimeout = 5 * time.Second
	ingestTimeout = 30 * time.Second
	queryTimeout  = 30 * time.Second

	jsonErrorMaxBytes = 4 * 1024
)

var (
	pkgMu      sync.Mutex
	pkgRefs    int
	pkgStarted bool
	pkgCLI     string

	startupMu sync.Mutex
	ingestMu  sync.Mutex

	unsupportedReason string
)

type Helper struct {
	cli       string
	startedAt time.Time

	mu     sync.Mutex
	closed bool
}

type TraceRecord = map[string]any
type MetricRecord = map[string]any
type LogRecord = map[string]any

func Start(t testing.TB) *Helper {
	t.Helper()
	if unsupportedReason != "" {
		t.Skip("lotelhelper: " + unsupportedReason)
		return nil
	}

	cli, err := acquireCLI()
	if err != nil {
		t.Skipf("lotelhelper: %v", err)
		return nil
	}
	if !ensureRunning(t, cli) {
		return nil
	}

	h := &Helper{cli: cli, startedAt: time.Now()}
	t.Cleanup(func() { h.cleanup(t) })
	return h
}

func (h *Helper) Endpoint() string { return OTLPGRPCEndpoint }

func (h *Helper) HTTPEndpoint() string { return OTLPHTTPEndpoint }

func (h *Helper) CLI() string { return h.cli }

func (h *Helper) Since() string {
	elapsed := time.Since(h.startedAt) + time.Second
	return strconv.Itoa(int(elapsed.Seconds())) + "s"
}

func (h *Helper) Ingest(t testing.TB) {
	t.Helper()
	ingestMu.Lock()
	defer ingestMu.Unlock()
	stdout, stderr, err := h.runCLI(ingestTimeout, "ingest")
	if err != nil {
		t.Fatalf("lotelhelper: ingest failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
}

func (h *Helper) QueryTraces(t testing.TB, service, since string) []TraceRecord {
	t.Helper()
	var out []TraceRecord
	h.query(t, "traces", service, since, &out)
	return out
}

func (h *Helper) QueryMetrics(t testing.TB, service, since string) []MetricRecord {
	t.Helper()
	var out []MetricRecord
	h.query(t, "metrics", service, since, &out)
	return out
}

func (h *Helper) QueryLogs(t testing.TB, service, since string) []LogRecord {
	t.Helper()
	var out []LogRecord
	h.query(t, "logs", service, since, &out)
	return out
}

func (h *Helper) WaitForTraces(t testing.TB, service string, minCount int, timeout time.Duration) []TraceRecord {
	t.Helper()
	return waitForRecords(t, service, minCount, timeout, h.Ingest, h.QueryTraces, h.Since, "traces")
}

func (h *Helper) WaitForMetrics(t testing.TB, service string, minCount int, timeout time.Duration) []MetricRecord {
	t.Helper()
	return waitForRecords(t, service, minCount, timeout, h.Ingest, h.QueryMetrics, h.Since, "metrics")
}

func (h *Helper) WaitForLogs(t testing.TB, service string, minCount int, timeout time.Duration) []LogRecord {
	t.Helper()
	return waitForRecords(t, service, minCount, timeout, h.Ingest, h.QueryLogs, h.Since, "logs")
}

func waitForRecords[T ~map[string]any](
	t testing.TB,
	service string,
	minCount int,
	timeout time.Duration,
	ingest func(testing.TB),
	query func(testing.TB, string, string) []T,
	since func() string,
	signal string,
) []T {
	t.Helper()
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	deadline := time.Now().Add(timeout)
	backoff := 250 * time.Millisecond
	var last []T
	for {
		ingest(t)
		last = query(t, service, since())
		if len(last) >= minCount {
			return last
		}
		if time.Now().After(deadline) {
			t.Fatalf("lotelhelper: WaitFor%s(%q, %d, %s): timed out; last poll returned %d records",
				signal, service, minCount, timeout, len(last))
			return last
		}
		time.Sleep(backoff)
		if backoff < 4*time.Second {
			backoff *= 2
			if backoff > 4*time.Second {
				backoff = 4 * time.Second
			}
		}
	}
}

func acquireCLI() (string, error) {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	if pkgCLI != "" {
		return pkgCLI, nil
	}
	cli, err := resolveCLI()
	if err != nil {
		return "", err
	}
	pkgCLI = cli
	return cli, nil
}

func ensureRunning(t testing.TB, cli string) bool {
	t.Helper()
	startupMu.Lock()
	defer startupMu.Unlock()

	pkgMu.Lock()
	first := pkgRefs == 0
	pkgMu.Unlock()

	if first {
		startedHere := false
		if collectorHealthy(cli) {
			pkgMu.Lock()
			pkgStarted = false
			pkgMu.Unlock()
		} else if err := startCollector(cli); err != nil {
			t.Skipf("lotelhelper: lotel-cli start failed: %v", err)
			return false
		} else {
			pkgMu.Lock()
			pkgStarted = true
			pkgMu.Unlock()
			startedHere = true
		}
		if _, stderr, err := runCmd(cli, ingestTimeout, "ingest"); err != nil {
			if startedHere {
				_, _, _ = runCmd(cli, stopTimeout, "stop")
				pkgMu.Lock()
				pkgStarted = false
				pkgMu.Unlock()
			}
			t.Skipf("lotelhelper: lotel-cli ingest unavailable after startup: %v\nstderr: %s", err, stderr)
			return false
		}
	}

	pkgMu.Lock()
	pkgRefs++
	pkgMu.Unlock()
	return true
}

func (h *Helper) query(t testing.TB, signal, service, since string, dst any) {
	t.Helper()
	args := []string{"query", signal}
	if service != "" {
		args = append(args, "--service", service)
	}
	if since != "" {
		args = append(args, "--since", since)
	}
	stdout, stderr, err := h.runCLI(queryTimeout, args...)
	if err != nil {
		t.Fatalf("lotelhelper: query %s failed: %v\nstdout: %s\nstderr: %s",
			signal, err, truncateForLog(stdout), stderr)
	}
	if err := json.Unmarshal(stdout, dst); err != nil {
		t.Fatalf("lotelhelper: query %s: decode JSON: %v\nstdout (%d bytes, truncated): %s\nstderr: %s",
			signal, err, len(stdout), truncateForLog(stdout), stderr)
	}
}

func (h *Helper) cleanup(t testing.TB) {
	t.Helper()
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	cli := h.cli
	h.mu.Unlock()

	pkgMu.Lock()
	pkgRefs--
	shouldStop := pkgRefs == 0 && pkgStarted
	pkgMu.Unlock()
	if !shouldStop {
		return
	}

	_, _, err := runCmd(cli, stopTimeout, "stop")
	pkgMu.Lock()
	if err == nil {
		pkgStarted = false
	}
	pkgMu.Unlock()
	if err != nil {
		t.Logf("lotelhelper: lotel-cli stop failed: %v", err)
	}
}

func (h *Helper) runCLI(timeout time.Duration, args ...string) ([]byte, []byte, error) {
	return runCmd(h.cli, timeout, args...)
}

func resolveCLI() (string, error) {
	if env := os.Getenv(envLotelCLI); env != "" {
		if _, err := os.Stat(env); err != nil { //nolint:gosec
			return "", fmt.Errorf("$%s=%s: %w", envLotelCLI, env, err)
		}
		return env, nil
	}
	if path, err := exec.LookPath("lotel-cli"); err == nil {
		return path, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		fallback := filepath.Join(home, fallbackCLIRel)
		if _, err := os.Stat(fallback); err == nil {
			return fallback, nil
		}
	}
	return "", errors.New("lotel-cli not found (set $LOTEL_CLI, install on $PATH, or build under ~/git/lotel)")
}

func collectorHealthy(cli string) bool {
	_, _, err := runCmd(cli, healthTimeout, "health")
	return err == nil
}

func startCollector(cli string) error {
	_, _, err := runCmd(cli, startTimeout, "start", "--wait")
	return err
}

func runCmd(cli string, timeout time.Duration, args ...string) (stdout, stderr []byte, err error) {
	if timeout <= 0 {
		timeout = startTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...) //nolint:gosec
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout, stderr, fmt.Errorf("%s %v: %w", filepath.Base(cli), args, ctxErr)
	}
	if runErr != nil {
		return stdout, stderr, fmt.Errorf("%s %v: %w", filepath.Base(cli), args, runErr)
	}
	return stdout, stderr, nil
}

func truncateForLog(b []byte) string {
	if len(b) <= jsonErrorMaxBytes {
		return string(b)
	}
	return string(b[:jsonErrorMaxBytes]) + "...[truncated]"
}

func init() {
	if runtime.GOOS == "windows" {
		unsupportedReason = "integration tests not supported on Windows"
	}
}
