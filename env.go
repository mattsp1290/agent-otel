package agentotel

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	EnvOTLPEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"
	EnvOTLPHeaders  = "OTEL_EXPORTER_OTLP_HEADERS"
	EnvOTLPInsecure = "OTEL_EXPORTER_OTLP_INSECURE"
	EnvOTLPProtocol = "OTEL_EXPORTER_OTLP_PROTOCOL"
	EnvOTLPTimeout  = "OTEL_EXPORTER_OTLP_TIMEOUT"
)

type lookupEnv func(string) (string, bool)

func processEnv(name string) (string, bool) {
	return os.LookupEnv(name)
}

func resolveProtocol(sig signal, explicit ExporterConfig, lookup lookupEnv) (protocol, field, value string) {
	if strings.TrimSpace(explicit.Protocol) != "" {
		return strings.TrimSpace(explicit.Protocol), optionField(sig, "protocol"), explicit.Protocol
	}
	if raw, ok := lookup(perSignalEnv(sig, "PROTOCOL")); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw), perSignalEnv(sig, "PROTOCOL"), raw
	}
	if raw, ok := lookup(EnvOTLPProtocol); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw), EnvOTLPProtocol, raw
	}
	return ProtocolGRPC, "default protocol", ProtocolGRPC
}

func resolveEndpoint(sig signal, explicit ExporterConfig, protocol string, lookup lookupEnv) (endpoint, field, value string) {
	if strings.TrimSpace(explicit.Endpoint) != "" {
		return strings.TrimSpace(explicit.Endpoint), optionField(sig, "endpoint"), explicit.Endpoint
	}
	if raw, ok := lookup(perSignalEnv(sig, "ENDPOINT")); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw), perSignalEnv(sig, "ENDPOINT"), raw
	}
	if raw, ok := lookup(EnvOTLPEndpoint); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw), EnvOTLPEndpoint, raw
	}
	if protocol == ProtocolHTTPProtobuf {
		return defaultHTTPEndpoint, "default endpoint", defaultHTTPEndpoint
	}
	return defaultGRPCEndpoint, "default endpoint", defaultGRPCEndpoint
}

func resolveInsecure(sig signal, explicit ExporterConfig, endpoint string, lookup lookupEnv) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "http://") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "https://") {
		return false
	}
	if explicit.Insecure {
		return true
	}
	if raw, ok := lookup(perSignalEnv(sig, "INSECURE")); ok && strings.TrimSpace(raw) != "" {
		return parseBool(raw, false)
	}
	if raw, ok := lookup(EnvOTLPInsecure); ok && strings.TrimSpace(raw) != "" {
		return parseBool(raw, false)
	}
	return false
}

func resolveHeaders(sig signal, explicit ExporterConfig, lookup lookupEnv) map[string]string {
	if len(explicit.Headers) > 0 {
		return cloneHeaders(explicit.Headers)
	}
	if raw, ok := lookup(perSignalEnv(sig, "HEADERS")); ok {
		return parseHeaders(raw)
	}
	if raw, ok := lookup(EnvOTLPHeaders); ok {
		return parseHeaders(raw)
	}
	return nil
}

func resolveTimeout(sig signal, explicit ExporterConfig, opts Options, lookup lookupEnv) (time.Duration, error) {
	if explicit.Timeout > 0 {
		return explicit.Timeout, nil
	}
	if raw, ok := lookup(perSignalEnv(sig, "TIMEOUT")); ok && strings.TrimSpace(raw) != "" {
		return parseTimeout(perSignalEnv(sig, "TIMEOUT"), raw)
	}
	if raw, ok := lookup(EnvOTLPTimeout); ok && strings.TrimSpace(raw) != "" {
		return parseTimeout(EnvOTLPTimeout, raw)
	}
	if opts.ExportTimeout > 0 {
		return opts.ExportTimeout, nil
	}
	return 0, nil
}

func normalizeEndpointForProtocol(field, raw, endpoint, protocol string, insecure bool) (string, bool, error) {
	switch protocol {
	case ProtocolGRPC:
		return normalizeGRPCEndpoint(field, raw, endpoint, insecure)
	case ProtocolHTTPProtobuf:
		return normalizeHTTPEndpoint(field, raw, endpoint, insecure)
	default:
		return "", false, validateProtocol(field, raw, protocol)
	}
}

func normalizeGRPCEndpoint(field, raw, endpoint string, insecure bool) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	lower := strings.ToLower(endpoint)
	switch {
	case strings.HasPrefix(lower, "http://"):
		endpoint = endpoint[len("http://"):]
		insecure = true
	case strings.HasPrefix(lower, "https://"):
		endpoint = endpoint[len("https://"):]
		insecure = false
	}
	if err := validateHostPortEndpoint(field, raw, endpoint); err != nil {
		return "", false, err
	}
	return endpoint, insecure, nil
}

func normalizeHTTPEndpoint(field, raw, endpoint string, insecure bool) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if !strings.Contains(endpoint, "://") {
		if err := validateHostPortEndpoint(field, raw, endpoint); err != nil {
			return "", false, err
		}
		if insecure {
			return "http://" + endpoint, true, nil
		}
		return "https://" + endpoint, false, nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", false, &ConfigValidationError{Field: field, Value: raw, Reason: "not a valid URL", Err: err}
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		insecure = true
	case "https":
		insecure = false
	default:
		return "", false, &ConfigValidationError{Field: field, Value: raw, Reason: fmt.Sprintf("unsupported scheme %q", u.Scheme)}
	}
	if u.Host == "" {
		return "", false, &ConfigValidationError{Field: field, Value: raw, Reason: "host portion is empty"}
	}
	if err := validateNoWhitespaceOrControl(field, raw, endpoint); err != nil {
		return "", false, err
	}
	if port := u.Port(); port != "" {
		if err := validatePort(field, raw, port); err != nil {
			return "", false, err
		}
	}
	return u.String(), insecure, nil
}

func validateProtocol(field, raw, protocol string) error {
	switch protocol {
	case ProtocolGRPC, ProtocolHTTPProtobuf:
		return nil
	default:
		return &ConfigValidationError{
			Field:  field,
			Value:  raw,
			Reason: fmt.Sprintf("unsupported protocol %q (expected %q or %q)", protocol, ProtocolGRPC, ProtocolHTTPProtobuf),
		}
	}
}

func validateHostPortEndpoint(field, raw, endpoint string) error {
	if endpoint == "" {
		return &ConfigValidationError{Field: field, Value: raw, Reason: "endpoint is empty"}
	}
	if err := validateNoWhitespaceOrControl(field, raw, endpoint); err != nil {
		return err
	}
	lower := strings.ToLower(endpoint)
	if strings.Contains(endpoint, "://") ||
		strings.HasPrefix(lower, "//") ||
		strings.HasPrefix(lower, "http:/") ||
		strings.HasPrefix(lower, "https:/") ||
		strings.HasPrefix(lower, "http/") ||
		strings.HasPrefix(lower, "https/") {
		return &ConfigValidationError{
			Field:  field,
			Value:  raw,
			Reason: `looks like a malformed scheme prefix`,
		}
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return &ConfigValidationError{Field: field, Value: raw, Reason: fmt.Sprintf("not a valid host:port (%v)", err), Err: err}
	}
	if host == "" {
		return &ConfigValidationError{Field: field, Value: raw, Reason: "host portion is empty"}
	}
	if err := validatePort(field, raw, port); err != nil {
		return err
	}
	return nil
}

func validateNoWhitespaceOrControl(field, raw, value string) error {
	for i, r := range value {
		if unicode.IsSpace(r) {
			return &ConfigValidationError{
				Field:  field,
				Value:  raw,
				Reason: fmt.Sprintf("contains whitespace at byte offset %d", i),
			}
		}
		if r < 0x20 || r == 0x7f {
			return &ConfigValidationError{
				Field:  field,
				Value:  raw,
				Reason: fmt.Sprintf("contains control character (U+%04X) at byte offset %d", r, i),
			}
		}
	}
	return nil
}

func validatePort(field, raw, port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return &ConfigValidationError{Field: field, Value: raw, Reason: fmt.Sprintf("port %q is not a number", port), Err: err}
	}
	if portNum < 1 || portNum > 65535 {
		return &ConfigValidationError{Field: field, Value: raw, Reason: fmt.Sprintf("port %d out of range (must be 1-65535)", portNum)}
	}
	return nil
}

func parseHeaders(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		eq := strings.IndexByte(pair, '=')
		if eq <= 0 {
			continue
		}
		key, err := url.QueryUnescape(strings.TrimSpace(pair[:eq]))
		if err != nil || key == "" {
			continue
		}
		value, err := url.QueryUnescape(strings.TrimSpace(pair[eq+1:]))
		if err != nil {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseBool(raw string, fallback bool) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if value, err := strconv.ParseBool(raw); err == nil {
		return value
	}
	switch strings.ToLower(raw) {
	case "y", "yes", "on":
		return true
	case "n", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseTimeout(field, raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &ConfigValidationError{Field: field, Value: raw, Reason: "timeout is not a millisecond integer", Err: err}
	}
	if ms < 0 {
		return 0, &ConfigValidationError{Field: field, Value: raw, Reason: "timeout must be non-negative"}
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func perSignalEnv(sig signal, suffix string) string {
	return "OTEL_EXPORTER_OTLP_" + strings.ToUpper(string(sig)) + "_" + suffix
}

func optionField(sig signal, name string) string {
	return "Options." + string(sig) + "." + name
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	return out
}
