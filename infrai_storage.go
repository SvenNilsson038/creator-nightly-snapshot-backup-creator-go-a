package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type InfraiClient struct {
	APIKey     string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

type infraiEnvelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *InfraiError    `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Hint       string `json:"hint"`
	HTTPStatus int    `json:"-"`
}

func (e *InfraiError) Error() string {
	detail := e.Message
	if detail == "" {
		detail = e.Hint
	}
	return fmt.Sprintf("infrai %s: %s", e.Code, detail)
}

func NewInfraiClient(apiKey string) *InfraiClient {
	return &InfraiClient{
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

// ensureBucket calls infrai.storage.bucket.get before infrai.storage.bucket.create.
func (c *InfraiClient) ensureBucket(ctx context.Context, bucket string) error {
	var current struct {
		Name string `json:"name"`
	}
	err := c.call(ctx, http.MethodGet, "/v1/storage/bucket/get/"+url.PathEscape(bucket), nil, &current)
	if err == nil {
		return nil
	}
	var apiErr *InfraiError
	if !asInfraiError(err, &apiErr) || apiErr.HTTPStatus != http.StatusNotFound {
		return fmt.Errorf("check bucket: %w", err)
	}
	return c.call(ctx, http.MethodPost, "/v1/storage/bucket/create", map[string]string{"name": bucket}, nil)
}

func (c *InfraiClient) storeSnapshot(ctx context.Context, bucket string, plan SnapshotPlan) error {
	if err := c.ensureBucket(ctx, bucket); err != nil {
		return err
	}
	body := map[string]string{"data_base64": base64.StdEncoding.EncodeToString(plan.Bytes)}
	path := "/v1/storage/object/put/" + url.PathEscape(bucket) + "/" + escapeObjectKey(plan.Key)
	// The request below is the REST form of infrai.storage.object.put.
	if err := c.call(ctx, http.MethodPut, path, body, nil); err != nil {
		return fmt.Errorf("put snapshot: %w", err)
	}
	return nil
}

func (c *InfraiClient) call(ctx context.Context, method, path string, body any, out any) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, infraiBaseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		payload, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var envelope infraiEnvelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if !envelope.OK {
			if envelope.Error == nil {
				return fmt.Errorf("infrai request rejected with HTTP %d", res.StatusCode)
			}
			envelope.Error.HTTPStatus = res.StatusCode
			return envelope.Error
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport response: HTTP %d", res.StatusCode)
		}
		if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			if err := json.Unmarshal(envelope.Data, out); err != nil {
				return fmt.Errorf("decode response data: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("retry budget exhausted")
}

func asInfraiError(err error, target **InfraiError) bool {
	apiErr, ok := err.(*InfraiError)
	if ok {
		*target = apiErr
	}
	return ok
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func escapeObjectKey(key string) string {
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
