package typesafe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "sk-test-sentinel-key"

func TestEvaluateSendsPinnedModelHeadersAndNoulBody(t *testing.T) {
	var captured methodCapture
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		captured.record(t, request)
		writeNoul(writer, map[string]float64{"gate": 0.9}, 11, 3)
	}))
	t.Cleanup(server.Close)

	client := testClient(t, server)
	response, err := client.Evaluate(context.Background(), Request{
		State: map[string]any{"task": "write tests"},
		Questions: map[string]Question{
			"gate": {
				Type:         questionTypeNoul,
				Instructions: "Is this a skill-worthy task?",
				Criteria:     &NoulCriteria{True: "needs a documented skill", False: "general knowledge"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPost {
		t.Fatalf("method = %q", captured.method)
	}
	if captured.path != evaluatePath {
		t.Fatalf("path = %q", captured.path)
	}
	if captured.authorization != "Bearer "+testKey {
		t.Fatalf("authorization = %q", captured.authorization)
	}
	if captured.contentType != "application/json" || captured.accept != "application/json" {
		t.Fatalf("content-type=%q accept=%q", captured.contentType, captured.accept)
	}
	if captured.userAgent != userAgent {
		t.Fatalf("user-agent = %q", captured.userAgent)
	}
	if captured.body["model"] != Model {
		t.Fatalf("model = %#v", captured.body["model"])
	}
	questions, _ := captured.body["questions"].(map[string]any)
	gate, _ := questions["gate"].(map[string]any)
	if gate["type"] != questionTypeNoul {
		t.Fatalf("question = %#v", gate)
	}
	criteria, _ := gate["criteria"].(map[string]any)
	if criteria["true"] != "needs a documented skill" {
		t.Fatalf("criteria = %#v", criteria)
	}
	if response.Model != Model || response.Answers["gate"].Noul == nil || *response.Answers["gate"].Noul != 0.9 {
		t.Fatalf("response = %#v", response)
	}
	if response.Usage.InputTokens != 11 || response.Usage.OutputTokens != 3 {
		t.Fatalf("usage = %#v", response.Usage)
	}
}

func TestEvaluateRejectsOversizedRequestsWithoutHTTP(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	t.Cleanup(server.Close)
	client := testClient(t, server)

	t.Run("byte cap", func(t *testing.T) {
		_, err := client.Evaluate(context.Background(), Request{
			State:     strings.Repeat("a", MaxRequestBytes+1),
			Questions: map[string]Question{"q": {Type: questionTypeNoul, Instructions: "x"}},
		})
		if ReasonOf(err) != ReasonRequestTooLarge {
			t.Fatalf("err = %v", err)
		}
		if hits.Load() != 0 {
			t.Fatalf("http hits = %d", hits.Load())
		}
	})

	t.Run("request token cap", func(t *testing.T) {
		// 128001 bytes estimate to 64000+ tokens while staying under MaxRequestBytes.
		_, err := client.Evaluate(context.Background(), Request{
			State:     strings.Repeat("b", 128_001),
			Questions: map[string]Question{"q": {Type: questionTypeNoul, Instructions: "x"}},
		})
		if ReasonOf(err) != ReasonRequestTooLarge {
			t.Fatalf("err = %v", err)
		}
		if hits.Load() != 0 {
			t.Fatalf("http hits = %d", hits.Load())
		}
	})

	t.Run("state plus longest question token cap", func(t *testing.T) {
		_, err := client.Evaluate(context.Background(), Request{
			State: strings.Repeat("c", 60_000),
			Questions: map[string]Question{
				"q": {Type: questionTypeNoul, Instructions: strings.Repeat("d", 5_000)},
			},
		})
		if ReasonOf(err) != ReasonRequestTooLarge {
			t.Fatalf("err = %v", err)
		}
		if hits.Load() != 0 {
			t.Fatalf("http hits = %d", hits.Load())
		}
	})
}

func TestEvaluateClassifiesHTTPStatuses(t *testing.T) {
	cases := []struct {
		status int
		reason string
	}{
		{http.StatusUnauthorized, ReasonInvalidKey},
		{http.StatusForbidden, ReasonInvalidKey},
		{http.StatusBadRequest, ReasonRequestRejected},
		{http.StatusRequestEntityTooLarge, ReasonRequestRejected},
		{http.StatusUnprocessableEntity, ReasonRequestRejected},
		{http.StatusTooManyRequests, ReasonRateLimited},
		{http.StatusInternalServerError, ReasonProviderError},
		{http.StatusServiceUnavailable, ReasonProviderOverloaded},
		{529, ReasonProviderOverloaded},
	}
	for _, test := range cases {
		t.Run(fmt.Sprintf("%d", test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(`{"error":"secret-body ` + testKey + `"}`))
			}))
			t.Cleanup(server.Close)
			_, err := testClient(t, server).Evaluate(context.Background(), pingRequest())
			if ReasonOf(err) != test.reason {
				t.Fatalf("reason = %q err=%v", ReasonOf(err), err)
			}
			message := err.Error()
			if strings.Contains(message, testKey) || strings.Contains(message, "secret-body") {
				t.Fatalf("leaked error %q", message)
			}
		})
	}
}

func TestEvaluateRejectsRedirectWithoutFollowing(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetHits.Add(1)
	}))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/v1/systemone", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	_, err := testClient(t, server).Evaluate(context.Background(), pingRequest())
	if ReasonOf(err) != ReasonUnexpectedRedirect {
		t.Fatalf("err = %v", err)
	}
	if targetHits.Load() != 0 {
		t.Fatalf("redirect target hits = %d", targetHits.Load())
	}
}

func TestEvaluateRejectsMalformedResponses(t *testing.T) {
	cases := map[string]string{
		"oversize":     strings.Repeat("x", maxResponseBytes+2),
		"trailing":     `{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1,"output_tokens":1}}{"extra":true}`,
		"missing":      `{"model":"jev-1.13.0","answers":{"ping":{"type":"noul"}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"out-of-range": `{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":1.2}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"negative":     `{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":-0.1}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"inf":          `{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":1e999}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"choice":       `{"model":"jev-1.13.0","answers":{"ping":{"type":"choice","choice":"a"}},"usage":{"input_tokens":1,"output_tokens":1}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(body))
			}))
			t.Cleanup(server.Close)
			_, err := testClient(t, server).Evaluate(context.Background(), pingRequest())
			if ReasonOf(err) != ReasonMalformedResponse {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestEvaluateTimeoutAndCancel(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			time.Sleep(200 * time.Millisecond)
			writeNoul(writer, map[string]float64{"ping": 1}, 1, 1)
		}))
		t.Cleanup(server.Close)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := testClient(t, server).Evaluate(ctx, pingRequest())
		if ReasonOf(err) != ReasonTimeout {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("cancel one attempt", func(t *testing.T) {
		var hits atomic.Int32
		started := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			hits.Add(1)
			close(started)
			time.Sleep(300 * time.Millisecond)
		}))
		t.Cleanup(server.Close)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := testClient(t, server).Evaluate(ctx, pingRequest())
			done <- err
		}()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("server did not receive the request")
		}
		cancel()
		err := <-done
		if ReasonOf(err) != ReasonCancelled {
			t.Fatalf("err = %v", err)
		}
		if hits.Load() != 1 {
			t.Fatalf("hits = %d", hits.Load())
		}
	})
}

func TestVerifyPingReturnsUsage(t *testing.T) {
	var captured methodCapture
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		captured.record(t, request)
		writeNoul(writer, map[string]float64{"ping": 1}, 7, 2)
	}))
	t.Cleanup(server.Close)

	result, err := testClient(t, server).Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if captured.body["state"] != "ping" {
		t.Fatalf("state = %#v", captured.body["state"])
	}
	if result.Model != Model || result.Usage.InputTokens != 7 || result.Usage.OutputTokens != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestKeyNeverLeaks(t *testing.T) {
	key := Key(testKey)
	printed := fmt.Sprintf("%v %s %+v %#v", key, key, key, key)
	if strings.Contains(printed, testKey) {
		t.Fatalf("printed key: %q", printed)
	}
	if !strings.Contains(printed, "[redacted]") {
		t.Fatalf("printed = %q", printed)
	}
	_, err := json.Marshal(key)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatalf("marshal error leaked: %v", err)
	}
}

func TestEstimateTokensIsBytesOverTwo(t *testing.T) {
	if got := EstimateTokens(9); got != 4 {
		t.Fatalf("EstimateTokens(9) = %d", got)
	}
}

func pingRequest() Request {
	return Request{
		State: "ping",
		Questions: map[string]Question{
			"ping": {Type: questionTypeNoul, Instructions: "Is the state exactly the word ping?"},
		},
	}
}

type methodCapture struct {
	method        string
	path          string
	authorization string
	contentType   string
	accept        string
	userAgent     string
	body          map[string]any
}

func (c *methodCapture) record(t *testing.T, request *http.Request) {
	t.Helper()
	c.method = request.Method
	c.path = request.URL.Path
	c.authorization = request.Header.Get("Authorization")
	c.contentType = request.Header.Get("Content-Type")
	c.accept = request.Header.Get("Accept")
	c.userAgent = request.Header.Get("User-Agent")
	data, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &c.body); err != nil {
		t.Fatal(err)
	}
}

func writeNoul(writer http.ResponseWriter, values map[string]float64, input, output int) {
	answers := map[string]any{}
	for id, value := range values {
		answers[id] = map[string]any{"type": questionTypeNoul, "noul": value}
	}
	payload, _ := json.Marshal(map[string]any{
		"model":   Model,
		"answers": answers,
		"usage":   map[string]int{"input_tokens": input, "output_tokens": output},
	})
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(payload)
}

func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	client := NewWithHTTP(Key(testKey), newHTTPClient(server.Client().Transport))
	client.origin = server.URL
	return client
}
