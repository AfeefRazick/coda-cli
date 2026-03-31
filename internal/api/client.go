package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AfeefRazick/coda-cli/internal/auth"
)

const baseURL = "https://coda.io/apis/v1"

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

type APIError struct {
	Method string
	Path   string
	Code   int
	Body   string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("coda api %s %s failed with status %d", e.Method, e.Path, e.Code)
	}
	return fmt.Sprintf("coda api %s %s failed with status %d: %s", e.Method, e.Path, e.Code, e.Body)
}

type MutationStatus struct {
	ID        string `json:"id"`
	RequestID string `json:"requestId"`
	Completed bool   `json:"completed"`
	Resource  struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"resource"`
	Error   string `json:"error"`
	Warning string `json:"warning"`
}

func NewClient() (*Client, string, error) {
	token, source, err := auth.ResolveToken()
	if err != nil {
		return nil, "", err
	}
	if token == "" {
		return nil, "", fmt.Errorf("no Coda API token found; set %s or run 'coda auth login'", auth.TokenEnvVar)
	}

	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		token:      token,
		baseURL:    baseURL,
	}, source, nil
}

func (c *Client) Request(ctx context.Context, method, rawPath string, query url.Values, body any) ([]byte, http.Header, int, error) {
	requestURL, err := c.buildURL(rawPath, query)
	if err != nil {
		return nil, nil, 0, err
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to encode request body: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, resp.StatusCode, &APIError{
			Method: method,
			Path:   rawPath,
			Code:   resp.StatusCode,
			Body:   strings.TrimSpace(string(respBody)),
		}
	}

	return respBody, resp.Header, resp.StatusCode, nil
}

func (c *Client) buildURL(rawPath string, query url.Values) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("empty API path")
	}

	if strings.HasPrefix(rawPath, "http://") || strings.HasPrefix(rawPath, "https://") {
		u, err := url.Parse(rawPath)
		if err != nil {
			return "", fmt.Errorf("invalid API path: %w", err)
		}
		if query != nil {
			merged := u.Query()
			for key, values := range query {
				for _, value := range values {
					merged.Add(key, value)
				}
			}
			u.RawQuery = merged.Encode()
		}
		return u.String(), nil
	}

	if !strings.HasPrefix(rawPath, "/") {
		rawPath = "/" + rawPath
	}

	u, err := url.Parse(c.baseURL + rawPath)
	if err != nil {
		return "", fmt.Errorf("invalid API path: %w", err)
	}
	if query != nil {
		merged := u.Query()
		for key, values := range query {
			for _, value := range values {
				merged.Add(key, value)
			}
		}
		u.RawQuery = merged.Encode()
	}

	return u.String(), nil
}

func (c *Client) WaitForMutation(ctx context.Context, requestID string, pollInterval time.Duration) (*MutationStatus, error) {
	for {
		body, _, _, err := c.Request(ctx, http.MethodGet, "/mutationStatus/"+url.PathEscape(requestID), nil, nil)
		if err != nil {
			return nil, err
		}

		var status MutationStatus
		if err := json.Unmarshal(body, &status); err != nil {
			return nil, fmt.Errorf("failed to parse mutation status: %w", err)
		}

		if status.Completed {
			return &status, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}
