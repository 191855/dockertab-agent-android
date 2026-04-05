package notifications

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordingSender struct {
	mu    sync.Mutex
	calls []senderCall
}

type senderCall struct {
	id, name, action string
}

func (r *recordingSender) Send(_ context.Context, id, name, action string) {
	r.mu.Lock()
	r.calls = append(r.calls, senderCall{id, name, action})
	r.mu.Unlock()
}

func (r *recordingSender) snapshot() []senderCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]senderCall(nil), r.calls...)
}

// TestDebouncer_StartNoPending: OnStart with no pending die → immediate "start".
func TestDebouncer_StartNoPending(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	d.OnStart(context.Background(), "ctr1", "myapp")
	calls := s.snapshot()
	if len(calls) != 1 || calls[0].action != "start" {
		t.Fatalf("want [start], got %v", calls)
	}
	if calls[0].id != "ctr1" || calls[0].name != "myapp" {
		t.Errorf("unexpected id/name: %+v", calls[0])
	}
}

// TestDebouncer_DieStartCollapsesToRestart: die then start within window → exactly one "restart".
func TestDebouncer_DieStartCollapsesToRestart(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	d.OnDie(context.Background(), "ctr1", "myapp")
	d.OnStart(context.Background(), "ctr1", "myapp")
	calls := s.snapshot()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d: %v", len(calls), calls)
	}
	if calls[0].action != "restart" {
		t.Errorf("want action=restart, got %q", calls[0].action)
	}
}

// TestDebouncer_DieStartNoExtraDie: "die" timer must be cancelled when "start" arrives.
func TestDebouncer_DieStartNoExtraDie(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	d.OnDie(context.Background(), "ctr1", "myapp")
	d.OnStart(context.Background(), "ctr1", "myapp")
	// Wait past the debounce window to confirm the timer was stopped.
	time.Sleep(1600 * time.Millisecond)
	calls := s.snapshot()
	if len(calls) != 1 || calls[0].action != "restart" {
		t.Errorf("want exactly [restart], got %v", calls)
	}
}

// TestDebouncer_DieFiresAfterTimeout: OnDie with no subsequent start → "die" after 1.5s.
func TestDebouncer_DieFiresAfterTimeout(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	d.OnDie(context.Background(), "ctr1", "myapp")
	time.Sleep(1600 * time.Millisecond)
	calls := s.snapshot()
	if len(calls) != 1 {
		t.Fatalf("want 1 call after timeout, got %d: %v", len(calls), calls)
	}
	if calls[0].action != "die" {
		t.Errorf("want action=die, got %q", calls[0].action)
	}
}

// TestDebouncer_MultipleOnDieResetsTimer: second die replaces the first timer; only one "die" fires.
func TestDebouncer_MultipleOnDieResetsTimer(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	d.OnDie(context.Background(), "ctr1", "myapp")
	time.Sleep(800 * time.Millisecond)
	d.OnDie(context.Background(), "ctr1", "myapp") // reset timer
	time.Sleep(1600 * time.Millisecond)
	calls := s.snapshot()
	if len(calls) != 1 || calls[0].action != "die" {
		t.Errorf("want exactly [die], got %v", calls)
	}
}

// TestDebouncer_CancelledContextSkipsSend: cancelled ctx prevents the timer callback from sending.
func TestDebouncer_CancelledContextSkipsSend(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	ctx, cancel := context.WithCancel(context.Background())
	d.OnDie(ctx, "ctr1", "myapp")
	cancel()
	time.Sleep(1600 * time.Millisecond)
	if calls := s.snapshot(); len(calls) != 0 {
		t.Errorf("want no calls with cancelled ctx, got %v", calls)
	}
}

// TestDebouncer_IndependentContainers: debounce state is per container ID.
func TestDebouncer_IndependentContainers(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	d.OnDie(context.Background(), "ctr1", "app1")
	d.OnStart(context.Background(), "ctr2", "app2") // different container, no pending die
	calls := s.snapshot()
	if len(calls) != 1 || calls[0].action != "start" || calls[0].id != "ctr2" {
		t.Errorf("want [start for ctr2], got %v", calls)
	}
}

// TestDebouncer_MultipleContainersIndependentTimers: each container has its own debounce state.
func TestDebouncer_MultipleContainersIndependentTimers(t *testing.T) {
	s := &recordingSender{}
	d := NewDebouncer(s)
	d.OnDie(context.Background(), "ctr1", "app1")
	d.OnDie(context.Background(), "ctr2", "app2")
	d.OnStart(context.Background(), "ctr1", "app1") // collapse ctr1 to restart
	// ctr2 die timer still pending; start it too
	d.OnStart(context.Background(), "ctr2", "app2")
	calls := s.snapshot()
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d: %v", len(calls), calls)
	}
	actions := map[string]string{}
	for _, c := range calls {
		actions[c.id] = c.action
	}
	if actions["ctr1"] != "restart" || actions["ctr2"] != "restart" {
		t.Errorf("want both restart, got %v", actions)
	}
}
