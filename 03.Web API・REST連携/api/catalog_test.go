package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
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

func TestInvoiceBuilderMatchesReportsNetNumericContract(t *testing.T) {
	name := fmt.Sprintf("fixture-%d", productFixtureID.Add(1))
	productFixtures.Store(name, map[int][]reportsweb.Map{
		1: {{"請求番号": "1", "お客様名": "パオ"}},
		2: {
			{"請求番号": "1", "品番": "A", "品名": "even-low", "数量": 2.25, "単価": 1900.5},
			{"請求番号": "1", "品番": "B", "品名": "even-high", "数量": "2.75", "単価": "1901.5"},
		},
	})
	defer productFixtures.Delete(name)
	db, err := sql.Open("catalog-product-test", name)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	builder, err := (&app{db: db, resources: productTestResources(t)}).buildInvoice()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := builder.Value()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"field3:1": "2.25", "field4:1": "1,900", "field5:1": "3,800", "field3:2": "2.75", "field4:2": "1,902", "field5:2": "5,706"}
	for _, raw := range doc["Pages"].([]any)[0].(reportsweb.Map)["Values"].([]any) {
		v := raw.(reportsweb.Map)
		key := fmt.Sprintf("%v:%v", v["Name"], v["Index"])
		if expected, ok := want[key]; ok && fmt.Sprint(v["Value"]) == expected {
			delete(want, key)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing invoice values: %v", want)
	}
	if _, err = invoiceMultiply(math.MaxInt64, 2); err == nil {
		t.Fatal("multiply overflow accepted")
	}
	if _, err = invoiceAdd(math.MaxInt64, 1); err == nil {
		t.Fatal("add overflow accepted")
	}
}

var productFixtures sync.Map
var productFixtureID atomic.Uint64

type productDriver struct{}
type productConn struct{ sheets map[int][]reportsweb.Map }
type productRows struct {
	rows  [][]driver.Value
	index int
}

func (productDriver) Open(name string) (driver.Conn, error) {
	v, ok := productFixtures.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown product fixture %s", name)
	}
	return &productConn{sheets: v.(map[int][]reportsweb.Map)}, nil
}
func (*productConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*productConn) Close() error                        { return nil }
func (*productConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (c *productConn) QueryContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
	sheet := int(args[1].Value.(int64))
	values := make([][]driver.Value, 0, len(c.sheets[sheet]))
	for _, row := range c.sheets[sheet] {
		raw, _ := json.Marshal(row)
		values = append(values, []driver.Value{raw})
	}
	return &productRows{rows: values}, nil
}
func (*productRows) Columns() []string { return []string{"row_data"} }
func (*productRows) Close() error      { return nil }
func (r *productRows) Next(dest []driver.Value) error {
	if r.index == len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func init() { sql.Register("catalog-product-test", productDriver{}) }

func productTestResources(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	// PR-330：Docker のビルド（README の手順）では、このサンプルのフォルダーだけがビルドの対象になり samples/php は無い。
	// サンプル自身の resources（帳票定義は PHP と同じもの）を先に使う。
	if info, err := os.Stat(filepath.Join(dir, "resources", "definitions")); err == nil && info.IsDir() {
		return filepath.Join(dir, "resources")
	}
	for {
		candidate := filepath.Join(dir, "samples", "php", "resources")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("samples/php/resources not found")
		}
		dir = parent
	}
}

func productDocument(t *testing.T, sheets map[int][]reportsweb.Map) reportsweb.Map {
	t.Helper()
	name := fmt.Sprintf("fixture-%d", productFixtureID.Add(1))
	productFixtures.Store(name, sheets)
	defer productFixtures.Delete(name)
	db, err := sql.Open("catalog-product-test", name)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	builder, err := (&app{db: db, resources: productTestResources(t)}).buildProducts()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := builder.Value()
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func productValues(doc reportsweb.Map, field string) []string {
	var values []string
	for _, rawPage := range doc["Pages"].([]any) {
		for _, rawValue := range rawPage.(reportsweb.Map)["Values"].([]any) {
			value := rawValue.(reportsweb.Map)
			if value["Name"] == field {
				values = append(values, fmt.Sprint(value["Value"]))
			}
		}
	}
	return values
}

func repeated(value string, count int) []string {
	values := make([]string, count)
	for i := range values {
		values[i] = value
	}
	return values
}

func productDetails(bigCodes, smallCodes []string) []reportsweb.Map {
	rows := make([]reportsweb.Map, len(bigCodes))
	for i := range rows {
		rows[i] = reportsweb.Map{"大分類コード": bigCodes[i], "小分類コード": smallCodes[i], "品番": fmt.Sprintf("ID%02d", i+1), "品名": "商品"}
	}
	return rows
}

func TestProductsSubtotalsUseCodeIdentity(t *testing.T) {
	tests := []struct {
		name                             string
		big, small                       []reportsweb.Map
		bigCodes, smallCodes, wantCounts []string
	}{
		{
			name:       "same names across big codes",
			big:        []reportsweb.Map{{"大分類コード": "1", "大分類名称": "Big A"}, {"大分類コード": "2", "大分類名称": "Big A"}},
			small:      []reportsweb.Map{{"大分類コード": "1", "小分類コード": "1", "小分類名称": "Small A"}, {"大分類コード": "2", "小分類コード": "1", "小分類名称": "Small A"}},
			bigCodes:   append(repeated("1", 9), repeated("2", 9)...),
			smallCodes: repeated("1", 18),
			wantCounts: []string{"9 冊", "9 冊", "9 冊", "9 冊"},
		},
		{
			name:       "different small codes within one big code",
			big:        []reportsweb.Map{{"大分類コード": "1", "大分類名称": "Big A"}},
			small:      []reportsweb.Map{{"大分類コード": "1", "小分類コード": "1", "小分類名称": "Small A"}, {"大分類コード": "1", "小分類コード": "2", "小分類名称": "Small A"}},
			bigCodes:   repeated("1", 18),
			smallCodes: append(repeated("1", 9), repeated("2", 9)...),
			wantCounts: []string{"9 冊", "9 冊", "18 冊"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := productDocument(t, map[int][]reportsweb.Map{1: tc.big, 2: tc.small, 3: productDetails(tc.bigCodes, tc.smallCodes)})
			if got := len(doc["Pages"].([]any)); got != 2 {
				t.Fatalf("pages = %d, want 2", got)
			}
			var ids, counts []string
			for _, value := range productValues(doc, "品番") {
				if len(value) >= 2 && value[:2] == "ID" {
					ids = append(ids, value)
				} else if value != "" {
					counts = append(counts, value)
				}
			}
			wantIDs := make([]string, 18)
			for i := range wantIDs {
				wantIDs[i] = fmt.Sprintf("ID%02d", i+1)
			}
			if !reflect.DeepEqual(ids, wantIDs) {
				t.Fatalf("detail order = %v", ids)
			}
			if !reflect.DeepEqual(counts, tc.wantCounts) {
				t.Fatalf("subtotal counts = %v, want %v", counts, tc.wantCounts)
			}
		})
	}
}
