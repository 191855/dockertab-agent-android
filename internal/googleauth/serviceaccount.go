// Package googleauth handles Google service account authentication for FCM.
package googleauth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenURL = "https://oauth2.googleapis.com/token"

// ServiceAccount holds parsed Google service account credentials.
type ServiceAccount struct {
	ClientEmail string
	PrivateKey  *rsa.PrivateKey
}

// LoadServiceAccount reads and parses a Google service account JSON key file.
// Download from Google Cloud Console → IAM & Admin → Service Accounts → Keys → Add Key → JSON.
func LoadServiceAccount(path string) (*ServiceAccount, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read service account file %q: %w", path, err)
	}

	var raw struct {
		Type        string `json:"type"`
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse service account JSON: %w", err)
	}
	if raw.Type != "service_account" {
		return nil, fmt.Errorf("expected type=service_account in key file, got %q", raw.Type)
	}

	block, _ := pem.Decode([]byte(raw.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("private_key in service account file is not valid PEM")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}

	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key must be RSA (got %T)", parsed)
	}

	return &ServiceAccount{
		ClientEmail: raw.ClientEmail,
		PrivateKey:  rsaKey,
	}, nil
}

// TokenSource fetches and caches OAuth2 access tokens for a service account scope.
type TokenSource struct {
	sa         *ServiceAccount
	scope      string
	mu         sync.Mutex
	cached     string
	cachedExp  time.Time
	httpClient *http.Client
}

// NewTokenSource creates a TokenSource for the given service account and OAuth2 scope.
func NewTokenSource(sa *ServiceAccount, scope string) *TokenSource {
	return &TokenSource{
		sa:         sa,
		scope:      scope,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Token returns a valid access token, fetching a new one when the cached token expires.
func (ts *TokenSource) Token() (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.cached != "" && time.Now().Add(5*time.Minute).Before(ts.cachedExp) {
		return ts.cached, nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   ts.sa.ClientEmail,
		"scope": ts.scope,
		"aud":   tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := jwtToken.SignedString(ts.sa.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("sign service account JWT: %w", err)
	}

	resp, err := ts.httpClient.PostForm(tokenURL, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {signed},
	})
	if err != nil {
		return "", fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" {
		return "", fmt.Errorf("parse token response: %w", err)
	}

	ts.cached = result.AccessToken
	ts.cachedExp = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return ts.cached, nil
}
