package discat

import (
	"context"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type MockHTTPClient struct {
	mu          sync.Mutex
	requests    []*http.Request
	response    *http.Response
	responseErr error
}

func (c *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	if c.responseErr != nil {
		return nil, c.responseErr
	}
	return c.response, nil
}

func TestProcessInputWithContext(t *testing.T) {
	mockClient := &MockHTTPClient{
		response: &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}

	config := &Config{
		WebhookURL: "http://test.com",
		OneLine:    true,
		MaxRetries: 0,
		Timeout:    1 * time.Second,
	}
	logger := NewLogger(false)

	ds := &DiscordSender{
		client:      mockClient,
		config:      config,
		logger:      logger,
		rateLimiter: NewRateLimiter(),
		metrics:     NewMetrics(),
		wg:          &sync.WaitGroup{},
	}

	input := "hello\nworld\n"
	reader := strings.NewReader(input)

	err := ds.ProcessInputWithContext(context.Background(), reader)
	if err != nil {
		t.Fatalf("ProcessInputWithContext failed: %v", err)
	}

	ds.wg.Wait() // Wait for all goroutines to finish

	mockClient.mu.Lock()
	defer mockClient.mu.Unlock()

	if len(mockClient.requests) != 2 {
		t.Fatalf("Expected 2 requests, got %d", len(mockClient.requests))
	}

	expectedBodies := []string{`{"content":"hello"}`, `{"content":"world"}`}
	actualBodies := []string{}
	for _, req := range mockClient.requests {
		body, _ := io.ReadAll(req.Body)
		trimmedBody := strings.TrimSpace(string(body))
		actualBodies = append(actualBodies, trimmedBody)
	}

	sort.Strings(expectedBodies)
	sort.Strings(actualBodies)

	for i := range expectedBodies {
		if expectedBodies[i] != actualBodies[i] {
			t.Errorf("Expected body %q, got %q", expectedBodies[i], actualBodies[i])
		}
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantMsgs []string
	}{
		{
			name:    "short message",
			content: "hello world",
			wantLen: 1,
			wantMsgs: []string{"hello world"},
		},
		{
			name:    "long message",
			content: strings.Repeat("a", MaxDiscordMessageLength+1),
			wantLen: 2,
			wantMsgs: []string{strings.Repeat("a", MaxDiscordMessageLength), "a\n"},
		},
		{
			name:    "empty message",
			content: "",
			wantLen: 1,
			wantMsgs: []string{""},
		},
		{
			name:    "exact length message",
			content: strings.Repeat("a", MaxDiscordMessageLength),
			wantLen: 1,
			wantMsgs: []string{strings.Repeat("a", MaxDiscordMessageLength)},
		},
	}

	ds := &DiscordSender{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := ds.splitMessage(tt.content)
			if len(msgs) != tt.wantLen {
				t.Errorf("splitMessage() len = %v, want %v", len(msgs), tt.wantLen)
			}

			// A simple way to compare, could be improved with a deep equal
			if len(msgs) == len(tt.wantMsgs) {
				for i, msg := range msgs {
					if msg != tt.wantMsgs[i] {
						t.Errorf("splitMessage() msg[%d] = %v, want %v", i, msg, tt.wantMsgs[i])
					}
				}
			}
		})
	}
}