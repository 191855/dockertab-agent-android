package notifications

import (
	"reflect"
	"testing"
)

func onlyRecord(t *testing.T, ts *TokenStore) DeviceRecord {
	t.Helper()
	all := ts.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 device, got %d", len(all))
	}
	return all[0]
}

func TestRegisterKeepsEventFilter(t *testing.T) {
	ts := NewTokenStore(t.TempDir())
	ts.Register("pixel-7", "fcm-token", []string{"die", "restart"})

	got := onlyRecord(t, ts)
	if got.Token != "fcm-token" {
		t.Fatalf("token = %q", got.Token)
	}
	if !reflect.DeepEqual(got.Events, []string{"die", "restart"}) {
		t.Errorf("events = %v", got.Events)
	}
}

func TestRegisterSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	NewTokenStore(dir).Register("pixel-7", "fcm-token", []string{"start"})

	got := onlyRecord(t, NewTokenStore(dir))
	if !reflect.DeepEqual(got.Events, []string{"start"}) {
		t.Errorf("events after reload = %v", got.Events)
	}
}

func TestUnregisterRemovesDevice(t *testing.T) {
	ts := NewTokenStore(t.TempDir())
	ts.Register("pixel-7", "fcm-token", nil)
	ts.Unregister("pixel-7")

	if n := len(ts.All()); n != 0 {
		t.Errorf("expected no devices, got %d", n)
	}
}
