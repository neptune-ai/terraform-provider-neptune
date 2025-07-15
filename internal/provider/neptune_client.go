package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// NeptuneTokenData represents the decoded Neptune token structure.
type NeptuneTokenData struct {
	APIAddress string `json:"api_address"`
	APIUrl     string `json:"api_url"`
	APIKey     string `json:"api_key"`
	Token      string
}

// OAuthToken represents Neptune OAuth access and refresh tokens.
type OAuthToken struct {
	AccessToken    string    `json:"access_token"`
	RefreshToken   string    `json:"refresh_token"`
	Username       string    `json:"username"`
	ExpirationTime time.Time `json:"expiration_time"`
	Mutex          sync.RWMutex
}

// IsExpired checks if the token is expired with a 30-second buffer.
func (t *OAuthToken) IsExpired() bool {
	if t == nil {
		return true
	}
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	return time.Now().Add(30 * time.Second).After(t.ExpirationTime)
}

// GetAccessToken safely returns the access token.
func (t *OAuthToken) GetAccessToken() string {
	if t == nil {
		return ""
	}
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	return t.AccessToken
}

// Update safely updates the token fields.
func (t *OAuthToken) Update(accessToken, refreshToken, username string, expirationTime time.Time) {
	if t == nil {
		return
	}
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	t.AccessToken = accessToken
	t.RefreshToken = refreshToken
	t.Username = username
	t.ExpirationTime = expirationTime
}

// GetRefreshToken safely returns the refresh token.
func (t *OAuthToken) GetRefreshToken() string {
	if t == nil {
		return ""
	}
	t.Mutex.RLock()
	defer t.Mutex.RUnlock()
	return t.RefreshToken
}

// NeptuneClient represents an authenticated Neptune API client.
type NeptuneClient struct {
	httpClient      *http.Client
	tokenData       *NeptuneTokenData
	providerVersion string
	oauthToken      *OAuthToken
	authMutex       sync.RWMutex
	workspace       string
}

// NeptuneError represents Neptune API error response.
type NeptuneError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Code       string `json:"code"`
	Type       string `json:"type"`
}

func (e *NeptuneError) Error() string {
	return fmt.Sprintf("Neptune API error [%d]: %s (code: %s, type: %s)", e.StatusCode, e.Message, e.Code, e.Type)
}

// OAuthTokenResponse represents the response from token exchange.
type OAuthTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Username     string `json:"username"`
}

// TokenRefreshRequest represents the request body for token refresh.
type TokenRefreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	ExpiresIn    int64  `json:"expires_in"`
}

// NewNeptuneClient creates a new Neptune client from a base64 encoded token.
func NewNeptuneClient(encodedToken string, workspace string, timeout int64, providerVersion string) (*NeptuneClient, error) {
	// Decode the base64 token
	tokenBytes, err := base64.StdEncoding.DecodeString(encodedToken)
	if err != nil {
		return nil, fmt.Errorf("neptune_token seems to be invalid. Please check your token and try again. (BASE64_DECODE_ERROR: %w)", err)
	}

	// Parse the token JSON
	var tokenData NeptuneTokenData
	if err := json.Unmarshal(tokenBytes, &tokenData); err != nil {
		return nil, fmt.Errorf("neptune_token seems to be invalid. Please check your token and try again. (JSON_UNMARSHAL_ERROR: %w)", err)
	}

	// Validate required fields in token data
	if tokenData.APIAddress == "" || tokenData.APIUrl == "" || tokenData.APIKey == "" {
		return nil, fmt.Errorf("neptune_token seems to be invalid. Please check your token and try again")
	}

	// If timeout is 0, set it to 30 seconds as a default
	if timeout == 0 {
		timeout = 30
	}

	// Store the encoded token in the token data for later use
	tokenData.Token = encodedToken

	return &NeptuneClient{
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		tokenData:       &tokenData,
		providerVersion: providerVersion,
		workspace:       workspace,
	}, nil
}

// parseJWTExpiration extracts expiration time from JWT token.
func parseJWTExpiration(tokenString string) (time.Time, error) {
	// Parse without verification (just to get claims)
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid JWT claims")
	}

	exp, ok := claims["exp"]
	if !ok {
		return time.Time{}, fmt.Errorf("no expiration claim in JWT")
	}

	var expTime time.Time
	switch exp := exp.(type) {
	case float64:
		expTime = time.Unix(int64(exp), 0)
	case json.Number:
		val, err := exp.Int64()
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid expiration time format: %w", err)
		}
		expTime = time.Unix(val, 0)
	default:
		return time.Time{}, fmt.Errorf("unexpected expiration time type: %T", exp)
	}

	return expTime, nil
}

// exchangeAPIKeyForToken exchanges the API key for OAuth tokens.
func (c *NeptuneClient) exchangeAPIKeyForToken(ctx context.Context) error {
	authEndpoint := c.tokenData.APIAddress + "/api/backend/v1/authorization/oauth-token"
	req, err := http.NewRequestWithContext(ctx, "GET", authEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create token exchange request: %w", err)
	}
	// Set the API token header
	req.Header.Set("X-Neptune-Api-Token", c.tokenData.Token)
	req.Header.Set("Accept", "application/json")

	parsedURL, err := url.Parse(c.tokenData.APIUrl)

	if err != nil {
		return fmt.Errorf("failed to parse authority host from token API URL: %w", err)
	}
	req.Header.Set("authority", parsedURL.Host)
	userAgent := fmt.Sprintf("terraform-provider-neptune/%s (%s %s)",
		c.providerVersion,
		runtime.GOOS,
		runtime.GOARCH)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)

	if err != nil {
		return fmt.Errorf("token exchange request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("neptune API key was rejected by Neptune backend (expired or invalid). Please check your token and try again")
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	// Parse expiration time from access token
	expTime, err := parseJWTExpiration(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to parse token expiration: %w", err)
	}

	// Update the OAuth token
	c.authMutex.Lock()
	if c.oauthToken == nil {
		c.oauthToken = &OAuthToken{}
	}
	c.oauthToken.Update(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.Username, expTime)
	c.authMutex.Unlock()

	return nil
}

// refreshToken refreshes the OAuth token using the refresh token.
func (c *NeptuneClient) refreshToken(ctx context.Context) error {
	c.authMutex.RLock()
	if c.oauthToken == nil {
		c.authMutex.RUnlock()
		return fmt.Errorf("no existing token to refresh")
	}
	refreshToken := c.oauthToken.GetRefreshToken()
	c.authMutex.RUnlock()

	if refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	// Calculate seconds left (can be negative if expired)
	secondsLeft := int64(time.Until(c.oauthToken.ExpirationTime).Seconds())

	refreshReq := TokenRefreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     c.tokenData.APIUrl,
		ExpiresIn:    secondsLeft,
	}

	jsonData, err := json.Marshal(refreshReq)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.tokenData.APIAddress+"/api/backend/v1/authorization/oauth-token", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	userAgent := fmt.Sprintf("terraform-provider-neptune/%s (%s %s)",
		c.providerVersion,
		runtime.GOOS,
		runtime.GOARCH)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode refresh response: %w", err)
	}

	// Parse expiration time from new access token
	expTime, err := parseJWTExpiration(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to parse refreshed token expiration: %w", err)
	}

	// Update the OAuth token
	c.authMutex.Lock()
	c.oauthToken.Update(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.Username, expTime)
	c.authMutex.Unlock()

	return nil
}

// ensureValidToken ensures we have a valid, non-expired OAuth token.
func (c *NeptuneClient) ensureValidToken(ctx context.Context) error {
	c.authMutex.RLock()
	hasToken := c.oauthToken != nil
	isExpired := c.oauthToken == nil || c.oauthToken.IsExpired()
	c.authMutex.RUnlock()

	if !hasToken {
		// No token yet, exchange API key
		return c.exchangeAPIKeyForToken(ctx)
	}

	if isExpired {
		// Token expired, try to refresh
		err := c.refreshToken(ctx)
		if err != nil {
			// If refresh fails, try to get a new token with API key
			c.authMutex.Lock()
			c.oauthToken = nil
			c.authMutex.Unlock()
			return c.exchangeAPIKeyForToken(ctx)
		}
	}

	return nil
}

// makeRequest performs an authenticated HTTP request to the Neptune API.
func (c *NeptuneClient) makeRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	// Ensure we have a valid token
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.tokenData.APIAddress + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header with OAuth token
	c.authMutex.RLock()
	accessToken := ""
	if c.oauthToken != nil {
		accessToken = c.oauthToken.GetAccessToken()
	}
	c.authMutex.RUnlock()

	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	userAgent := fmt.Sprintf("terraform-provider-neptune/%s (%s %s)",
		c.providerVersion,
		runtime.GOOS,
		runtime.GOARCH)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Handle error responses
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, &NeptuneError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("HTTP %d: failed to read error response", resp.StatusCode),
				Type:       "UNKNOWN_ERROR",
			}
		}

		// Try to parse Neptune error format
		var neptuneErr struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}

		if json.Unmarshal(bodyBytes, &neptuneErr) == nil && neptuneErr.Error.Message != "" {
			return nil, &NeptuneError{
				StatusCode: resp.StatusCode,
				Message:    neptuneErr.Error.Message,
				Code:       neptuneErr.Error.Code,
				Type:       neptuneErr.Error.Type,
			}
		}

		// Fallback to generic error
		message := string(bodyBytes)
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}

		return nil, &NeptuneError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Type:       getErrorTypeFromStatusCode(resp.StatusCode),
		}
	}

	return resp, nil
}

// getErrorTypeFromStatusCode maps HTTP status codes to Neptune error types.
func getErrorTypeFromStatusCode(statusCode int) string {
	switch statusCode {
	case 400:
		return "BAD_REQUEST"
	case 401:
		return "UNAUTHORIZED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 405:
		return "METHOD_NOT_ALLOWED"
	case 409:
		return "CONFLICT"
	case 422:
		return "UNPROCESSABLE_ENTITY"
	case 429:
		return "RATE_LIMITED"
	case 500:
		return "INTERNAL_SERVER_ERROR"
	case 502:
		return "BAD_GATEWAY"
	case 503:
		return "SERVICE_UNAVAILABLE"
	case 504:
		return "GATEWAY_TIMEOUT"
	default:
		return "UNKNOWN_ERROR"
	}
}

// Get performs a GET request to the Neptune API.
func (c *NeptuneClient) Get(ctx context.Context, endpoint string) (*http.Response, error) {
	return c.makeRequest(ctx, "GET", endpoint, nil)
}

// Post performs a POST request to the Neptune API.
func (c *NeptuneClient) Post(ctx context.Context, endpoint string, body interface{}) (*http.Response, error) {
	return c.makeRequest(ctx, "POST", endpoint, body)
}

// Put performs a PUT request to the Neptune API.
func (c *NeptuneClient) Put(ctx context.Context, endpoint string, body interface{}) (*http.Response, error) {
	return c.makeRequest(ctx, "PUT", endpoint, body)
}

// Delete performs a DELETE request to the Neptune API.
func (c *NeptuneClient) Delete(ctx context.Context, endpoint string) (*http.Response, error) {
	return c.makeRequest(ctx, "DELETE", endpoint, nil)
}
