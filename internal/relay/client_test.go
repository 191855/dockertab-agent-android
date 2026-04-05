package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/dockertab/agent-android/internal/docker"
)

type routingDocker struct {
	mu     sync.Mutex
	called string
	rows   int
	cols   int
}

func (m *routingDocker) record(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.called == "" {
		m.called = method
	}
}

func (m *routingDocker) firstCalled() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

func (m *routingDocker) StreamLogs(_ context.Context, _ string, _ int) (io.ReadCloser, error) {
	m.record("StreamLogs")
	return io.NopCloser(strings.NewReader("")), fmt.Errorf("stub")
}

func (m *routingDocker) GetContainerStats(_ context.Context, _ string) (*docker.ContainerStats, error) {
	m.record("GetContainerStats")
	return nil, fmt.Errorf("stub")
}

func (m *routingDocker) ExecCreate(_ context.Context, _ string, _ []string, rows, cols int) (string, error) {
	m.record("ExecCreate")
	m.mu.Lock()
	m.rows, m.cols = rows, cols
	m.mu.Unlock()
	return "", fmt.Errorf("stub")
}

func (m *routingDocker) Ping(_ context.Context) error                                               { return nil }
func (m *routingDocker) Close() error                                                               { return nil }
func (m *routingDocker) ListContainers(_ context.Context) ([]docker.ContainerSummary, error)        { return nil, nil }
func (m *routingDocker) GetContainer(_ context.Context, _ string) (*docker.ContainerSummary, error) { return nil, nil }
func (m *routingDocker) GetContainerLogs(_ context.Context, _ string, _ int) (string, error)       { return "", nil }
func (m *routingDocker) GetHostInfo(_ context.Context) (*docker.HostInfo, error)                    { return nil, nil }
func (m *routingDocker) ListImages(_ context.Context) ([]docker.ImageSummary, error)                { return nil, nil }
func (m *routingDocker) StartContainer(_ context.Context, _ string) error                           { return nil }
func (m *routingDocker) StopContainer(_ context.Context, _ string) error                            { return nil }
func (m *routingDocker) RestartContainer(_ context.Context, _ string) error                         { return nil }
func (m *routingDocker) GetContainerEnv(_ context.Context, _ string) ([]string, error)              { return nil, nil }
func (m *routingDocker) ExecAttach(_ context.Context, _ string) (types.HijackedResponse, error)    { return types.HijackedResponse{}, fmt.Errorf("stub") }
func (m *routingDocker) ExecResize(_ context.Context, _ string, _, _ int) error                    { return nil }
func (m *routingDocker) Events(_ context.Context) (<-chan docker.ContainerEvent, <-chan error)       { return make(chan docker.ContainerEvent), make(chan error) }
func (m *routingDocker) ListVolumes(_ context.Context) ([]docker.VolumeSummary, error)              { return nil, nil }
func (m *routingDocker) CheckImageUpdates(_ context.Context, _ []string) (map[string]bool, error)   { return map[string]bool{}, nil }

func newRoutingClient(d docker.DockerClient) *Client {
	return &Client{
		docker:  d,
		streams: make(map[string]*streamEntry),
		send:    make(chan []byte, 256),
	}
}

func streamEnv(path string) Envelope {
	p, _ := json.Marshal(RequestPayload{Method: "GET", Path: path})
	return Envelope{
		Type:      TypeStreamOpen,
		RequestID: "req-1",
		ClientID:  "client-1",
		Payload:   p,
	}
}

func cancelledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestHandleStreamOpen_RoutesToLogs(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(cancelledCtx(), streamEnv("/api/v1/containers/abc123/logs/stream"))
	if got := md.firstCalled(); got != "StreamLogs" {
		t.Errorf("want StreamLogs, got %q", got)
	}
}

func TestHandleStreamOpen_RoutesToStats(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(cancelledCtx(), streamEnv("/api/v1/containers/abc123/stats/stream"))
	if got := md.firstCalled(); got == "StreamLogs" || got == "ExecCreate" {
		t.Errorf("stats path: unexpected call to %q", got)
	}
}

func TestHandleStreamOpen_RoutesToExec(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(cancelledCtx(), streamEnv("/api/v1/containers/abc123/exec"))
	if got := md.firstCalled(); got != "ExecCreate" {
		t.Errorf("want ExecCreate, got %q", got)
	}
}

func TestHandleStreamOpen_ExecWithQueryParams(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(cancelledCtx(), streamEnv("/api/v1/containers/abc123/exec?rows=30&cols=120"))
	if got := md.firstCalled(); got != "ExecCreate" {
		t.Fatalf("want ExecCreate, got %q", got)
	}
	md.mu.Lock()
	rows, cols := md.rows, md.cols
	md.mu.Unlock()
	if rows != 30 {
		t.Errorf("want rows=30, got %d", rows)
	}
	if cols != 120 {
		t.Errorf("want cols=120, got %d", cols)
	}
}

func TestHandleStreamOpen_ExecDefaultDimensions(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(cancelledCtx(), streamEnv("/api/v1/containers/abc123/exec"))
	md.mu.Lock()
	rows, cols := md.rows, md.cols
	md.mu.Unlock()
	if rows != 24 {
		t.Errorf("want default rows=24, got %d", rows)
	}
	if cols != 80 {
		t.Errorf("want default cols=80, got %d", cols)
	}
}

func TestHandleStreamOpen_ContainerNameContainsLogsStream(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(
		cancelledCtx(),
		streamEnv("/api/v1/containers/my-logs-stream-svc/exec"),
	)
	if got := md.firstCalled(); got != "ExecCreate" {
		t.Errorf("want ExecCreate, got %q", got)
	}
}

func TestHandleStreamOpen_ContainerNameContainsStatsStream(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(
		cancelledCtx(),
		streamEnv("/api/v1/containers/my-stats-stream-svc/exec"),
	)
	if got := md.firstCalled(); got != "ExecCreate" {
		t.Errorf("want ExecCreate, got %q", got)
	}
}

func TestHandleStreamOpen_ContainerNameContainsExecRoutesToLogs(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(
		cancelledCtx(),
		streamEnv("/api/v1/containers/exec-runner/logs/stream"),
	)
	if got := md.firstCalled(); got != "StreamLogs" {
		t.Errorf("want StreamLogs, got %q", got)
	}
}

func TestHandleStreamOpen_QueryStringDoesNotAffectRouting(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(
		cancelledCtx(),
		streamEnv("/api/v1/containers/abc123/logs/stream?tail=100"),
	)
	if got := md.firstCalled(); got != "StreamLogs" {
		t.Errorf("want StreamLogs, got %q", got)
	}
}

func TestHandleStreamOpen_UnknownPathSuffix(t *testing.T) {
	md := &routingDocker{}
	newRoutingClient(md).handleStreamOpen(cancelledCtx(), streamEnv("/api/v1/containers/abc123/unknown"))
	if got := md.firstCalled(); got != "" {
		t.Errorf("want no docker call, got %q", got)
	}
}

func TestHandleStreamOpen_InvalidContainerID(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"dot-dot traversal", "/api/v1/containers/../../../etc/passwd/exec"},
		{"leading dot", "/api/v1/containers/.hidden/exec"},
		{"empty segment", "/api/v1/containers//exec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md := &routingDocker{}
			newRoutingClient(md).handleStreamOpen(cancelledCtx(), streamEnv(tc.path))
			if got := md.firstCalled(); got != "" {
				t.Errorf("want no docker call, got %q", got)
			}
		})
	}
}

func TestHandleStreamOpen_MalformedPayload(t *testing.T) {
	md := &routingDocker{}
	env := Envelope{
		Type:      TypeStreamOpen,
		RequestID: "req-1",
		Payload:   []byte("not-json"),
	}
	newRoutingClient(md).handleStreamOpen(cancelledCtx(), env)
	if got := md.firstCalled(); got != "" {
		t.Errorf("want no docker call, got %q", got)
	}
}

// TestHandleStreamInput_NoStream: input for an unknown request ID is silently dropped.
func TestHandleStreamInput_NoStream(t *testing.T) {
	c := newRoutingClient(&routingDocker{})
	env := Envelope{
		Type:      TypeStreamInput,
		RequestID: "unknown",
		Payload:   mustMarshalPayload(t, StreamPayload{Data: "aGVsbG8="}), // base64("hello")
	}
	// Must not panic.
	c.handleStreamInput(env)
}

// TestHandleStreamInput_NilStdin: input for a known stream with no exec attached is silently dropped.
func TestHandleStreamInput_NilStdin(t *testing.T) {
	c := newRoutingClient(&routingDocker{})
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.streamsMu.Lock()
	c.streams["req-1"] = &streamEntry{clientID: "client-1", cancel: cancel}
	c.streamsMu.Unlock()

	env := Envelope{
		Type:      TypeStreamInput,
		RequestID: "req-1",
		Payload:   mustMarshalPayload(t, StreamPayload{Data: "aGVsbG8="}),
	}
	// stdin is nil — must not panic.
	c.handleStreamInput(env)
}

// TestHandleStreamResize_NoStream: resize for an unknown request ID is silently dropped.
func TestHandleStreamResize_NoStream(t *testing.T) {
	md := &routingDocker{}
	c := newRoutingClient(md)
	env := Envelope{
		Type:      TypeStreamResize,
		RequestID: "unknown",
		Payload:   mustMarshalPayload(t, ResizePayload{Rows: 40, Cols: 120}),
	}
	c.handleStreamResize(env)
	// ExecResize must not have been called.
	if got := md.firstCalled(); got != "" {
		t.Errorf("want no docker call, got %q", got)
	}
}

// TestHandleStreamResize_EmptyExecID: resize for a stream with no exec session is silently dropped.
func TestHandleStreamResize_EmptyExecID(t *testing.T) {
	md := &routingDocker{}
	c := newRoutingClient(md)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.streamsMu.Lock()
	c.streams["req-1"] = &streamEntry{clientID: "client-1", cancel: cancel} // execID == ""
	c.streamsMu.Unlock()

	env := Envelope{
		Type:      TypeStreamResize,
		RequestID: "req-1",
		Payload:   mustMarshalPayload(t, ResizePayload{Rows: 40, Cols: 120}),
	}
	c.handleStreamResize(env)
	if got := md.firstCalled(); got != "" {
		t.Errorf("want no docker call, got %q", got)
	}
}

func mustMarshalPayload(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return data
}
