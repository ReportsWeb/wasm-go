package reportsweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultEngineURL is used when NewEngine gets "" and REPORTS_ENGINE_URL is not set.
const DefaultEngineURL = "http://127.0.0.1:3107"

// MaxBytes is the largest print data or PDF exchanged with the engine (32 MiB).
const MaxBytes = 32 * 1024 * 1024

// Engine is a client for the Reports.Web engine (ghcr.io/reportsweb/engine).
type Engine struct {
	URL      string        // engine base URL
	Timeout  time.Duration // per request; default 90 s
	MaxBytes int64         // largest PDF accepted; default 32 MiB
	Client   *http.Client  // optional; a client with Timeout is used when nil
}

// EngineError is returned when the engine answers with an error or cannot be reached.
// Status is the HTTP status (0 for transport errors).
type EngineError struct {
	Status  int
	Message string
}

func (e *EngineError) Error() string { return e.Message }

// NewEngine returns a client for url. An empty url means REPORTS_ENGINE_URL,
// or DefaultEngineURL when that is not set.
func NewEngine(url string) *Engine {
	if url == "" {
		url = os.Getenv("REPORTS_ENGINE_URL")
	}
	if url == "" {
		url = DefaultEngineURL
	}
	return &Engine{URL: strings.TrimRight(url, "/"), Timeout: 90 * time.Second, MaxBytes: MaxBytes}
}

func (e *Engine) client() *http.Client {
	if e.Client != nil {
		return e.Client
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("redirects are not followed")
	}}
}

// Health calls GET /health and returns the engine status, e.g. {"status":"ok","engine":"C++ WebAssembly"}.
func (e *Engine) Health(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.URL+"/health", nil)
	if err != nil {
		return nil, err
	}
	res, err := e.client().Do(req)
	if err != nil {
		return nil, &EngineError{Message: fmt.Sprintf("cannot reach the Reports.Web engine at %s: %v", e.URL, err)}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, &EngineError{Status: res.StatusCode, Message: fmt.Sprintf("engine health check failed (HTTP %d)", res.StatusCode)}
	}
	var v map[string]any
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// RenderPDF sends PREPEJ print data to POST /render/pdf and returns the PDF bytes.
// data may be a *Builder, a Map (or any value encodable as JSON), JSON text as a string, or []byte.
func (e *Engine) RenderPDF(ctx context.Context, data any) ([]byte, error) {
	var payload []byte
	var err error
	switch v := data.(type) {
	case *Builder:
		payload, err = v.JSON(false)
	case []byte:
		payload = v
	case string:
		payload = []byte(v)
	default:
		payload, err = json.Marshal(v)
	}
	if err != nil {
		return nil, err
	}
	if len(payload) > MaxBytes {
		return nil, &EngineError{Message: "print data exceeds 32 MiB"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL+"/render/pdf", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client().Do(req)
	if err != nil {
		return nil, &EngineError{Message: fmt.Sprintf("cannot reach the Reports.Web engine at %s: %v", e.URL, err)}
	}
	defer res.Body.Close()
	limit := e.MaxBytes
	if limit <= 0 {
		limit = MaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, &EngineError{Status: res.StatusCode, Message: err.Error()}
	}
	if res.StatusCode/100 != 2 {
		detail := strings.TrimSpace(string(body))
		var problem struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &problem) == nil && problem.Error != "" {
			detail = problem.Error
		}
		return nil, &EngineError{Status: res.StatusCode, Message: fmt.Sprintf("engine could not create the PDF (HTTP %d): %s", res.StatusCode, detail)}
	}
	if int64(len(body)) > limit {
		return nil, &EngineError{Status: res.StatusCode, Message: fmt.Sprintf("PDF exceeds %d bytes", limit)}
	}
	if !bytes.HasPrefix(body, []byte("%PDF-")) {
		return nil, &EngineError{Status: res.StatusCode, Message: "engine response is not a PDF"}
	}
	return body, nil
}
