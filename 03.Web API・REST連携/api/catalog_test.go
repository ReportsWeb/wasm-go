package main

import (
	"testing"

	"github.com/ReportsWeb/wasm-go/reportsweb"
)

func TestJSONStringNumberUsesPlainDecimal(t *testing.T) {
	row := reportsweb.Map{"請求番号": float64(10000001), "大分類コード": float64(10)}
	if got := s(row, "請求番号"); got != "10000001" {
		t.Fatalf("invoice number = %q", got)
	}
	if got := s(row, "大分類コード"); got != "10" {
		t.Fatalf("product category = %q", got)
	}
	if got := integer(s(reportsweb.Map{"数量": float64(12)}, "数量")); got != 12 {
		t.Fatalf("quantity = %d", got)
	}
	if got := number(int64(5630184)); got != "5,630,184" {
		t.Fatalf("computed amount = %q", got)
	}
}
