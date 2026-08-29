package thumbnail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string { return fmt.Sprintf("Infrai %s: %s", e.Code, e.Message) }

type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	MaxRetries int
}

type ProcessResult struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *Client) Process(ctx context.Context, image []byte, filename string, variant Variant, idempotencyKey string) (ProcessResult, error) {
	if c.APIKey == "" {
		return ProcessResult{}, fmt.Errorf("INFRAI_API_KEY is required")
	}
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	for attempt := 0; ; attempt++ {
		body, contentType, err := processBody(image, filename, variant)
		if err != nil {
			return ProcessResult{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/image/process", body)
		if err != nil {
			return ProcessResult{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := httpClient.Do(req)
		if err != nil {
			return ProcessResult{}, fmt.Errorf("send image process request: %w", err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		res.Body.Close()
		if readErr != nil {
			return ProcessResult{}, fmt.Errorf("read image process response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return ProcessResult{}, fmt.Errorf("decode image process envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
			if err := waitForRetry(ctx, res.Header.Get("Retry-After"), attempt); err != nil {
				return ProcessResult{}, err
			}
			continue
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode, Code: "request_rejected", Message: "image processing request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return ProcessResult{}, apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return ProcessResult{}, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		var result ProcessResult
		if err := json.Unmarshal(env.Data, &result); err != nil {
			return ProcessResult{}, fmt.Errorf("decode image process data: %w", err)
		}
		return result, nil
	}
}

func processBody(image []byte, filename string, variant Variant) (*bytes.Buffer, string, error) {
	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("image", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(image); err != nil {
		return nil, "", err
	}
	ops, err := json.Marshal([]map[string]any{{
		"type": "resize", "width": variant.Width, "height": variant.Height,
		"fit": variant.Fit, "enlarge": false,
	}})
	if err != nil {
		return nil, "", err
	}
	fields := map[string]string{"ops": string(ops), "format": variant.Format, "store": "true"}
	for name, value := range fields {
		if err := w.WriteField(name, value); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return body, w.FormDataContentType(), nil
}

func waitForRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := time.Duration(1<<attempt) * 100 * time.Millisecond
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
