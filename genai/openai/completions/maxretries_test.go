package completions

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// TestMaxRetries pins the wire behavior of MaxRetries against a consistently
// 503ing backend: nil uses the SDK default (3 total attempts), pointer to 0
// makes one attempt.
func TestMaxRetries(t *testing.T) {
	zero := 0
	cases := []struct {
		name       string
		maxRetries *int
		wantCalls  int32
	}{
		{name: "nil uses openai-go default (2 retries = 3 calls)", maxRetries: nil, wantCalls: 3},
		{name: "zero disables retries (1 call)", maxRetries: &zero, wantCalls: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusServiceUnavailable)
				io.WriteString(w, `{"error":{"message":"overloaded"}}`)
			}))
			defer srv.Close()

			req := &model.LLMRequest{
				Config: &genai.GenerateContentConfig{},
				Contents: []*genai.Content{
					{Role: "user", Parts: []*genai.Part{{Text: "hello"}}},
				},
			}

			cfg := Config{
				BaseURL: srv.URL, APIKey: "test-key", ModelName: "gpt-test",
				HTTPOptions: HTTPOptions{MaxRetries: tc.maxRetries},
			}
			m := New(cfg)

			for _, err := range m.GenerateContent(context.Background(), req, false) {
				_ = err // every attempt fails with 503; we only care about the call count
			}
			if got := atomic.LoadInt32(&calls); got != tc.wantCalls {
				t.Errorf("backend hit %d times, want %d (maxRetries=%v)", got, tc.wantCalls, tc.maxRetries)
			}
		})
	}
}
