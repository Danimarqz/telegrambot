package testutil

import (
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// FakeResponse represents a queued HTTP response returned by the fake client.
type FakeResponse struct {
	StatusCode int
	Body       string
}

// CapturedRequest keeps the Telegram endpoint invoked and the encoded parameters.
type CapturedRequest struct {
	Endpoint string
	Values   url.Values
}

// FakeHTTPClient captures outgoing requests and returns queued responses.
type FakeHTTPClient struct {
	mu        sync.Mutex
	responses []FakeResponse
	requests  []CapturedRequest
}

// NewFakeHTTPClient constructs a FakeHTTPClient with the provided responses.
func NewFakeHTTPClient(responses ...FakeResponse) *FakeHTTPClient {
	return &FakeHTTPClient{responses: append([]FakeResponse(nil), responses...)}
}

// Do stores the request and returns the next queued response.
func (f *FakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	bodyBytes, _ := io.ReadAll(req.Body)
	closeErr := req.Body.Close()
	if closeErr != nil {
		return nil, closeErr
	}

	values, _ := url.ParseQuery(string(bodyBytes))
	endpoint := path.Base(req.URL.Path)

	f.mu.Lock()
	f.requests = append(f.requests, CapturedRequest{
		Endpoint: endpoint,
		Values:   values,
	})

	var resp FakeResponse
	if len(f.responses) > 0 {
		resp = f.responses[0]
		f.responses = f.responses[1:]
	} else {
		resp = FakeResponse{
			StatusCode: http.StatusOK,
			Body:       `{"ok":true,"result":{"message_id":1}}`,
		}
	}
	f.mu.Unlock()

	return &http.Response{
		StatusCode: resp.StatusCode,
		Body:       io.NopCloser(strings.NewReader(resp.Body)),
		Header:     make(http.Header),
	}, nil
}

// Requests returns a snapshot of the captured requests.
func (f *FakeHTTPClient) Requests() []CapturedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]CapturedRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

// NewFakeBot builds a BotAPI instance wired to the fake HTTP client.
func NewFakeBot(responses ...FakeResponse) (*tgbotapi.BotAPI, *FakeHTTPClient) {
	client := NewFakeHTTPClient(responses...)
	bot := &tgbotapi.BotAPI{
		Token:  "token",
		Client: client,
		Buffer: 100,
		Self: tgbotapi.User{
			ID:       1,
			UserName: "serverbot",
		},
	}
	bot.SetAPIEndpoint(tgbotapi.APIEndpoint)
	return bot, client
}
