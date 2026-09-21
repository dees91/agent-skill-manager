package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// Model is the pinned System One version used for every request.
	Model = "jev-1.13.0"
	// Origin is the fixed production HTTPS origin.
	Origin            = "https://api.typesafe.ai"
	evaluatePath      = "/v1/systemone"
	MaxRequestBytes   = 256 << 10
	maxResponseBytes  = 1 << 20
	maxErrorBytes     = 4 << 10
	maxRequestTokens  = 64_000
	maxStateTokens    = 32_000
	httpClientTimeout = 12 * time.Second
	userAgent         = "Skill-Manager/advisor"
	questionTypeNoul  = "noul"
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Key is a TypeSafe API key that never prints or JSON-encodes its value.
type Key string

func (k Key) String() string { return "[redacted]" }

func (k Key) GoString() string { return "[redacted]" }

func (k Key) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[redacted]")
}

func (k Key) MarshalJSON() ([]byte, error) {
	return nil, errors.New("typesafe.Key cannot be marshaled")
}

// NoulCriteria is the live API object form of a Noul rubric.
type NoulCriteria struct {
	True  string `json:"true,omitempty"`
	False string `json:"false,omitempty"`
}

// Question is a Noul question. Only type "noul" is sent.
type Question struct {
	Type         string        `json:"type"`
	Instructions string        `json:"instructions"`
	Criteria     *NoulCriteria `json:"criteria,omitempty"`
}

// Request is the advisor-owned evaluation payload. Model is added at send time.
type Request struct {
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Answer is a Noul answer. Confidence is ignored when present.
type Answer struct {
	Type       string   `json:"type"`
	Noul       *float64 `json:"noul"`
	Confidence *float64 `json:"confidence"`
}

// Usage reports billed input tokens and free output tokens.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Response is a validated System One evaluation result.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// VerifyResult is the cheapest successful connectivity check.
type VerifyResult struct {
	Model string
	Usage Usage
}

type wireRequest struct {
	Model     string              `json:"model"`
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Client calls the TypeSafe System One HTTP API.
type Client struct {
	key    Key
	http   httpDoer
	origin string
}

// New returns a production client pinned to Origin.
func New(key Key) *Client {
	return NewWithHTTP(key, newHTTPClient(http.DefaultTransport))
}

// NewWithHTTP returns a client that uses doer for tests and injected transports.
func NewWithHTTP(key Key, doer httpDoer) *Client {
	if doer == nil {
		doer = newHTTPClient(http.DefaultTransport)
	}
	return &Client{key: key, http: doer, origin: Origin}
}

func newHTTPClient(rt http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout:   httpClientTimeout,
		Transport: rt,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// EstimateTokens returns a pessimistic token count of bytes/2.
func EstimateTokens(size int) int {
	if size < 0 {
		return 0
	}
	return size / 2
}

// Evaluate sends one System One request and returns a validated Noul response.
func (c *Client) Evaluate(ctx context.Context, request Request) (Response, error) {
	body, err := c.marshalRequest(request)
	if err != nil {
		return Response{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.origin, "/")+evaluatePath, bytes.NewReader(body))
	if err != nil {
		return Response{}, typedError(ReasonNetworkError, 0)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+string(c.key))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("User-Agent", userAgent)

	response, err := c.http.Do(httpRequest)
	if err != nil {
		return Response{}, classifyTransport(err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 && response.StatusCode < 400 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxErrorBytes))
		return Response{}, typedError(ReasonUnexpectedRedirect, response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxErrorBytes))
		return Response{}, typedError(classifyStatus(response.StatusCode), response.StatusCode)
	}

	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Response{}, classifyTransport(err)
	}
	if len(data) > maxResponseBytes {
		return Response{}, typedError(ReasonMalformedResponse, response.StatusCode)
	}
	decoded, err := decodeResponse(data)
	if err != nil {
		return Response{}, err
	}
	return decoded, nil
}

// Verify makes the cheapest valid Noul call.
func (c *Client) Verify(ctx context.Context) (VerifyResult, error) {
	response, err := c.Evaluate(ctx, Request{
		State: "ping",
		Questions: map[string]Question{
			"ping": {
				Type:         questionTypeNoul,
				Instructions: "Is the state exactly the word ping?",
			},
		},
	})
	if err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{Model: response.Model, Usage: response.Usage}, nil
}

func (c *Client) marshalRequest(request Request) ([]byte, error) {
	payload := wireRequest{Model: Model, State: request.State, Questions: request.Questions}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, typedError(ReasonRequestTooLarge, 0)
	}
	if len(body) > MaxRequestBytes {
		return nil, typedError(ReasonRequestTooLarge, 0)
	}
	if EstimateTokens(len(body)) > maxRequestTokens {
		return nil, typedError(ReasonRequestTooLarge, 0)
	}
	stateBytes, err := json.Marshal(request.State)
	if err != nil {
		return nil, typedError(ReasonRequestTooLarge, 0)
	}
	longestQuestion := 0
	for _, question := range request.Questions {
		encoded, err := json.Marshal(question)
		if err != nil {
			return nil, typedError(ReasonRequestTooLarge, 0)
		}
		if len(encoded) > longestQuestion {
			longestQuestion = len(encoded)
		}
	}
	if EstimateTokens(len(stateBytes)+longestQuestion) > maxStateTokens {
		return nil, typedError(ReasonRequestTooLarge, 0)
	}
	return body, nil
}

func decodeResponse(data []byte) (Response, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	var decoded Response
	if err := decoder.Decode(&decoded); err != nil {
		return Response{}, typedError(ReasonMalformedResponse, 0)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Response{}, typedError(ReasonMalformedResponse, 0)
	}
	if decoded.Answers == nil {
		return Response{}, typedError(ReasonMalformedResponse, 0)
	}
	for _, answer := range decoded.Answers {
		if err := validateNoul(answer); err != nil {
			return Response{}, err
		}
	}
	return decoded, nil
}

func validateNoul(answer Answer) error {
	if answer.Type != questionTypeNoul {
		return typedError(ReasonMalformedResponse, 0)
	}
	if answer.Noul == nil {
		return typedError(ReasonMalformedResponse, 0)
	}
	value := *answer.Noul
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return typedError(ReasonMalformedResponse, 0)
	}
	return nil
}

func classifyStatus(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ReasonInvalidKey
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return ReasonRequestRejected
	case http.StatusTooManyRequests:
		return ReasonRateLimited
	case http.StatusServiceUnavailable, 529:
		return ReasonProviderOverloaded
	default:
		return ReasonProviderError
	}
}

func classifyTransport(err error) error {
	if errors.Is(err, context.Canceled) {
		return typedError(ReasonCancelled, 0)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return typedError(ReasonTimeout, 0)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return typedError(ReasonTimeout, 0)
	}
	return typedError(ReasonNetworkError, 0)
}
