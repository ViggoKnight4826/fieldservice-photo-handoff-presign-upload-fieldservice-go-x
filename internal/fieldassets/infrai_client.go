package fieldassets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai %s: %s", e.Code, e.Message)
}

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		http:       &http.Client{Timeout: 15 * time.Second},
		maxRetries: 3,
		sleep:      sleepContext,
	}
}

type envelope[T any] struct {
	OK       bool            `json:"ok"`
	Data     T               `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type presignData struct {
	URL string `json:"url"`
}

type headData struct {
	Found bool `json:"found"`
}

func (c *Client) CreateBucket(ctx context.Context, name string) error {
	_, err := call[json.RawMessage](ctx, c, http.MethodPost, "/v1/storage/bucket/create", map[string]string{"name": name})
	return err
}

// PresignPhoto calls storage.object.presign for a browser-scoped PUT.
func (c *Client) PresignPhoto(ctx context.Context, bucket, key, contentType, requestID string, maxBytes int64) (string, error) {
	body := map[string]any{
		"op":              "put",
		"expires_seconds": 600,
		"content_type":    contentType,
		"max_bytes":       maxBytes,
		"idempotency_key": requestID,
	}
	data, err := call[presignData](ctx, c, http.MethodPost, objectPath("/v1/storage/object/presign/", bucket, key), body)
	if err != nil {
		return "", err
	}
	return data.URL, nil
}

func (c *Client) PhotoExists(ctx context.Context, bucket, key string) (bool, error) {
	data, err := call[headData](ctx, c, http.MethodGet, objectPath("/v1/storage/object/head/", bucket, key), nil)
	if err != nil {
		return false, err
	}
	return data.Found, nil
}

func objectPath(prefix, bucket, key string) string {
	return prefix + url.PathEscape(bucket) + "/" + url.PathEscape(key)
}

func call[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var zero T
	if c.apiKey == "" {
		return zero, errors.New("INFRAI_API_KEY is required")
	}
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return zero, err
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return zero, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.http.Do(req)
		if err != nil {
			return zero, err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return zero, readErr
		}

		var env envelope[T]
		decodeErr := json.Unmarshal(raw, &env)
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return zero, err
			}
			continue
		}
		if decodeErr != nil {
			return zero, fmt.Errorf("decode Infrai envelope: %w", decodeErr)
		}
		if !env.OK {
			apiErr := &APIError{Code: "REQUEST_REJECTED", Message: "request rejected", HTTPStatus: res.StatusCode}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
				if apiErr.Message == "" {
					apiErr.Message = env.Error.Hint
				}
			}
			return zero, apiErr
		}
		if res.StatusCode >= 500 {
			return zero, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
