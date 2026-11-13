package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func refreshAs(h *Handler, deviceID, deviceName string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/refresh", nil)
	c.Set("device_id", deviceID)
	c.Set("device_name", deviceName)
	h.Refresh(c)
	return w
}

func TestRefreshReturnsToken(t *testing.T) {
	h := newTestHandler(t, &mockDocker{})

	w := refreshAs(h, "pixel-7", "Pixel 7")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Token == "" {
		t.Fatal("token is empty")
	}
}

func TestRefreshKeepsDeviceIdentity(t *testing.T) {
	h := newTestHandler(t, &mockDocker{})

	var body struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(refreshAs(h, "pixel-7", "Pixel 7").Body.Bytes(), &body)

	claims, err := h.Auth.ValidateToken(body.Token)
	if err != nil {
		t.Fatalf("refreshed token does not validate: %v", err)
	}
	if claims.DeviceID != "pixel-7" || claims.DeviceName != "Pixel 7" {
		t.Errorf("got device %q / %q", claims.DeviceID, claims.DeviceName)
	}
}
