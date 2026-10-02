package reportsweb

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Live tests run only when REPORTS_ENGINE_URL points to an engine:
//
//	docker run -d -p 3107:3107 ghcr.io/reportsweb/engine:1.0.1
//	REPORTS_ENGINE_URL=http://127.0.0.1:3107 go test ./...
func liveEngine(t *testing.T) *Engine {
	if os.Getenv("REPORTS_ENGINE_URL") == "" {
		t.Skip("REPORTS_ENGINE_URL is not set")
	}
	return NewEngine("")
}

func quickDefinition() Map {
	return Map{"Version": "13.2.0", "Title": "package test", "Paper": 9, "CoordinateUnit": "mm", "Objects": []any{
		Map{"Name": "Message", "Kind": 0, "X": 20, "Y": 20, "Width": 150, "Height": 12, "Text": "", "FontName": "ＭＳ ゴシック", "FontSizePt": 14},
	}}
}

func TestEngineRendersPDF(t *testing.T) {
	e := liveEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	h, err := e.Health(ctx)
	if err != nil || h["status"] != "ok" {
		t.Fatalf("health: %v %v", h, err)
	}
	b := New()
	if err = b.SetDefinition(quickDefinition()); err != nil {
		t.Fatal(err)
	}
	_ = b.PageStart(nil)
	_ = b.SetValue("Message", "Reports.Web")
	_ = b.PageEnd()
	pdf, err := e.RenderPDF(ctx, b)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) || len(pdf) < 1000 {
		t.Fatalf("render: %d bytes, %v", len(pdf), err)
	}
}

func TestEngineErrorCarriesStatus(t *testing.T) {
	e := liveEngine(t)
	_, err := e.RenderPDF(context.Background(), "{}")
	var ee *EngineError
	if !errors.As(err, &ee) || ee.Status != 400 {
		t.Fatalf("want EngineError 400, got %v", err)
	}
}

func TestUnreachableEngine(t *testing.T) {
	e := NewEngine("http://127.0.0.1:1")
	e.Timeout = 2 * time.Second
	_, err := e.Health(context.Background())
	var ee *EngineError
	if !errors.As(err, &ee) || ee.Status != 0 {
		t.Fatalf("want transport EngineError, got %v", err)
	}
}
