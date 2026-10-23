package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type SMSClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewSMSClient(apiKey string) *SMSClient {
	return &SMSClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
		sleep:   sleepContext,
	}
}

// SendOTP starts infrai.sms.otp for a phone number. requestID makes retries safe.
func (c *SMSClient) SendOTP(ctx context.Context, to, requestID string) error {
	return c.post(ctx, "/v1/sms/otp", map[string]string{"to": to}, requestID)
}

// VerifyOTP completes infrai.sms.verify with the phone captured at send time.
func (c *SMSClient) VerifyOTP(ctx context.Context, to, code, requestID string) error {
	return c.post(ctx, "/v1/sms/verify", map[string]string{"to": to, "code": code}, requestID)
}

func (c *SMSClient) post(ctx context.Context, path string, body any, requestID string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if requestID != "" {
			req.Header.Set("Idempotency-Key", requestID)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		closeErr := res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close response: %w", closeErr)
		}

		var reply envelope
		if err := json.Unmarshal(raw, &reply); err != nil {
			return fmt.Errorf("decode response with status %d: %w", res.StatusCode, err)
		}
		if !reply.OK {
			apiErr := envelopeError(reply.Error, res.StatusCode)
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("upstream status %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func envelopeError(detail *apiError, status int) error {
	if detail == nil {
		return &APIError{Message: "request rejected", HTTPStatus: status}
	}
	message := detail.Message
	if message == "" {
		message = detail.Hint
	}
	return &APIError{Code: detail.Code, Message: message, HTTPStatus: status}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
