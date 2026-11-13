package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/191855/dockertab-agent-android/internal/googleauth"
)

const (
	fcmSendURL = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	fcmScope   = "https://www.googleapis.com/auth/firebase.messaging"
)

type TokenExpiredError struct {
	DeviceToken string
	Reason      string
}

func (e *TokenExpiredError) Error() string {
	return fmt.Sprintf("FCM token expired: %s", e.Reason)
}

type FCMClient struct {
	projectID   string
	tokenSource *googleauth.TokenSource
	httpClient  *http.Client
}

func NewFCMClient(projectID, serviceAccountFile string) (*FCMClient, error) {
	sa, err := googleauth.LoadServiceAccount(serviceAccountFile)
	if err != nil {
		return nil, err
	}
	return &FCMClient{
		projectID:   projectID,
		tokenSource: googleauth.NewTokenSource(sa, fcmScope),
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

type fcmMessage struct {
	Message fcmMessageBody `json:"message"`
}

type fcmMessageBody struct {
	Token        string            `json:"token"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
	Android      fcmAndroidConfig  `json:"android"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroidConfig struct {
	Priority string `json:"priority"`
}

func (c *FCMClient) Push(ctx context.Context, deviceToken, title, body, containerID, containerName, agentID string) error {
	accessToken, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	msg := fcmMessage{Message: fcmMessageBody{
		Token:        deviceToken,
		Notification: fcmNotification{Title: title, Body: body},
		Data: map[string]string{
			"container_id":   containerID,
			"container_name": containerName,
			"agent_id":       agentID,
		},
		Android: fcmAndroidConfig{Priority: "HIGH"},
	}}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal FCM payload: %w", err)
	}

	url := fmt.Sprintf(fcmSendURL, c.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create FCM request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("FCM HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var fcmErr struct {
			Error struct {
				Code    int    `json:"code"`
				Status  string `json:"status"`
				Message string `json:"message"`
				Details []struct {
					Type      string `json:"@type"`
					ErrorCode string `json:"errorCode"`
				} `json:"details"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&fcmErr)
		log.Printf("[FCM] push failed: status=%d reason=%s token=%.16s...", resp.StatusCode, fcmErr.Error.Status, deviceToken)

		for _, d := range fcmErr.Error.Details {
			if d.ErrorCode == "UNREGISTERED" {
				return &TokenExpiredError{DeviceToken: deviceToken, Reason: d.ErrorCode}
			}
		}
		return fmt.Errorf("FCM %d: %s", resp.StatusCode, fcmErr.Error.Message)
	}

	return nil
}
