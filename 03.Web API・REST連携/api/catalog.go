package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ReportsWeb/wasm-go/reportsweb"
)

func (a *app) catalogPrintData(sample string) (any, error) {
	var b *reportsweb.Builder
	var err error
	switch sample {
	case "multiples-of-ten":
		b, err = a.buildMultiples()
	case "postal":
		b, err = a.buildPostal()
	case "estimate":
		b, err = a.buildEstimate()
	case "invoice":
		b, err = a.buildInvoice()
	case "products":
		b, err = a.buildProducts()
	default:
		b, err = a.buildSimple(sample)
	}
	if err != nil {
		return nil, err
	}
	return b.Value()
}

func (a *app) definitionFile(name string) (reportsweb.Map, error) {
	v, err := readJSON(filepath.Join(a.resources, "definitions", name))
	if err != nil {
		return nil, err
	}
	d, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid definition %s", name)
	}
	if err = reportsweb.ValidateDefinition(d); err != nil {
		return nil, err
	}
	if unit, _ := d["CoordinateUnit"].(string); unit != "" && !strings.EqualFold(unit, "mm") {
		return nil, fmt.Errorf("original definition must use mm")
	}
	return d, nil
}

func (a *app) sampleDefinition(sample string) (reportsweb.Map, error) {
	if sample == "postal" {
		return a.definitionFile("postal-1.prepdj")
	}
	return a.definitionFile(sample + ".prepdj")
}

func (a *app) rows(sample string, sheet int) ([]reportsweb.Map, error) {
	rows, err := a.db.Query(`SELECT row_data::text FROM reports_framework_rows WHERE sample_key=$1 AND sheet_no=$2 ORDER BY row_no`, sample, sheet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []reportsweb.Map{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var row reportsweb.Map
		if err = json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if sample == "products" && sheet == 3 {
		sort.SliceStable(result, func(i, j int) bool {
			ai, aj := num(s(result[i], "大分類コード")), num(s(result[j], "大分類コード"))
			if ai != aj {
				return ai < aj
			}
			return num(s(result[i], "小分類コード")) < num(s(result[j], "小分類コード"))
		})
	}
	return result, nil
}

func (a *app) buildSimple(sample string) (*reportsweb.Builder, error) {
	d, e := a.sampleDefinition(sample)
	if e != nil {
		return nil, e
	}
	b := reportsweb.New()
	if e = b.SetDefinition(d); e != nil {
		return nil, e
	}
	if e = b.PageStart(nil); e != nil {
		return nil, e
	}
	if sample == "quick-start" {
		if e = b.SetValue("Text2", "Webブラウザで作った\n印刷データです。"); e != nil {
			return nil, e
		}
	}
	e = b.PageEnd()
	return b, e
}

func (a *app) buildMultiples() (*reportsweb.Builder, error) {
	d, e := a.sampleDefinition("multiples-of-ten")
	if e != nil {
		return nil, e
	}
	b := reportsweb.New()
	if e = b.SetDefinition(d); e != nil {
		return nil, e
	}
	for page := 1; page <= 4; page++ {
		if e = b.PageStart(nil); e != nil {
			return nil, e
		}
		b.SetValue("日付", now(false))
		b.SetValue("頁数", fmt.Sprintf("Page - %d", page))
		b.SetValue("フォントサイズ", "フォントサイズ\n 変更後")
		b.ChangeAttributes("フォントサイズ", reportsweb.Map{"fontSize": 12}, 0)
		// 2ページ目だけ、ページ上部の線「Line3」を非表示にする。空文字と drawing=false を指定する。
		if page == 2 {
			b.SetValueAt("Line3", "", 0, false)
		}
		for line := 0; line < 15; line++ {
			i := (page-1)*15 + line + 1
			b.SetValueAt("行番号", i, line, true)
			b.SetValueAt("10倍数", i*10, line, true)
			b.SetValueAt("横線", "", line, true)
			// 100で割り切れる値だけ、この行の文字色を青にする。
			if (i*10)%100 == 0 {
				b.ChangeAttributes("10倍数", reportsweb.Map{"foreground": "#FF0000FF"}, line)
			}
		}
		if e = b.PageEnd(); e != nil {
			return nil, e
		}
	}
	return b, nil
}

func (a *app) buildPostal() (*reportsweb.Builder, error) {
	r, e := a.rows("postal", 1)
	if e != nil {
		return nil, e
	}
	d1, e := a.definitionFile("postal-1.prepdj")
	if e != nil {
		return nil, e
	}
	d2, e := a.definitionFile("postal-2.prepdj")
	if e != nil {
		return nil, e
	}
	b := reportsweb.New()
	b.SetDefinition(d1)
	for start, page := 0, 0; start < len(r); start, page = start+32, page+1 {
		end := start + 32
		if end > len(r) {
			end = len(r)
		}
		d := d1
		if page >= 5 {
			d = d2
		}
		b.PageStart(d)
		b.SetValue("ページ", fmt.Sprintf("Page-%d", page+1))
		b.SetValue("日時", now(false))
		for i, row := range r[start:end] {
			v := orderedValues(row)
			b.SetValueAt("郵便番号", at(v, 0), i, true)
			b.SetValueAt("市区町村", at(v, 1), i, true)
			b.SetValueAt("住所", at(v, 2), i, true)
			b.SetValueAt("横罫線", "", i, true)
			if page >= 5 && i%2 == 1 {
				b.SetValueAt("網掛け", "", i, true)
			}
		}
		if page < 5 && start < end {
			v := orderedValues(r[start])
			b.SetValue("QR", strings.TrimSpace(at(v, 0)+" "+at(v, 1)+at(v, 2)))
		}
		b.PageEnd()
	}
	return b, nil
}

func (a *app) buildEstimate() (*reportsweb.Builder, error) {
	hs, e := a.rows("estimate", 1)
	if e != nil {
		return nil, e
	}
	ds, e := a.rows("estimate", 2)
	if e != nil {
		return nil, e
	}
	cover, e := a.definitionFile("estimate-cover.prepdj")
	if e != nil {
		return nil, e
	}
	body, e := a.definitionFile("estimate.prepdj")
	if e != nil {
		return nil, e
	}
	b := reportsweb.New()
	b.SetDefinition(cover)
	for _, h := range hs {
		b.PageStart(cover)
		b.SetValue("お客様名", s(h, "お客様名"))
		b.SetValue("担当者名", s(h, "担当者名"))
		b.PageEnd()
		b.PageStart(body)
		b.SetValue("見積番号", s(h, "見積番号"))
		b.SetValue("お客様名", s(h, "お客様名"))
		b.SetValue("担当者名", s(h, "担当者名"))
		b.SetValue("見積日", japaneseDate(s(h, "見積日")))
		b.SetValue("ヘッダ合計", "\\ "+number(h["合計金額"]))
		b.SetValue("消費税額", number(h["消費税額"]))
		b.SetValue("フッタ合計", number(h["合計金額"]))
		for i := 0; i <= 6; i++ {
			for _, n := range []string{"品番白", "品名白", "数量白", "単価白", "金額白", "品番青", "品名青", "数量青", "単価青", "金額青"} {
				b.SetValueAt(n, "", i, true)
			}
		}
		i := 0
		for _, r := range ds {
			if s(r, "見積番号") != s(h, "見積番号") {
				continue
			}
			b.SetValueAt("品番", s(r, "品番"), i, true)
			b.SetValueAt("品名", s(r, "品名"), i, true)
			b.SetValueAt("数量", s(r, "数量"), i, true)
			b.SetValueAt("単価", number(r["単価"]), i, true)
			b.SetValueAt("金額", number(r["金額"]), i, true)
			i++
		}
		b.PageEnd()
	}
	return b, nil
}

func (a *app) buildInvoice() (*reportsweb.Builder, error) {
	hs, e := a.rows("invoice", 1)
	if e != nil {
		return nil, e
	}
	ds, e := a.rows("invoice", 2)
	if e != nil {
		return nil, e
	}
	d, e := a.sampleDefinition("invoice")
	if e != nil {
		return nil, e
	}
	b := reportsweb.New()
	b.SetDefinition(d)
	px := 96.0 / 25.4
	adjust := []float64{-5, 44, -20, -10, -9}
	for _, h := range hs {
		current := matching(ds, "請求番号", s(h, "請求番号"))
		maxH := max(4, repeat(object(d, "hLine"))-1)
		maxV := max(1, repeat(object(d, "vLine"))-1)
		b.PageStart(nil)
		b.SetValue("txtNo", s(h, "請求番号"))
		b.SetValue("txtCustomer", s(h, "お客様名"))
		b.SetValue("txtDate", now(true))
		img, er := os.ReadFile(filepath.Join(a.resources, "images", "kakuin.png"))
		if er != nil {
			return nil, er
		}
		b.SetImage("Image1", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(img))
		xs := make([]float64, maxV)
		next := 0.0
		for j := 0; j < maxV; j++ {
			o := object(d, fmt.Sprintf("field%d", j+1))
			if j == 0 {
				xs[j] = f(o, "X") * px
			} else {
				xs[j] = next
			}
			next = xs[j] + (f(o, "Width")+adjust[j])*px
		}
		for i := 0; i < maxH; i++ {
			b.SetValueAt("hLine", "", i, true)
			b.SetValueAt("LineRect", "", i, true)
			if i == 0 {
				b.ChangeAttributes("hLine", reportsweb.Map{"borderWidth": .5 * px}, i)
			}
			if i == 1 {
				b.ChangeAttributes("hLine", reportsweb.Map{"strokeStyle": "Double"}, i)
			}
			color := "#FFFFFFB4"
			if i == 0 {
				color = "#FFFFDAB9"
			} else if i < maxH-3 {
				if i%2 == 1 {
					color = "#FFFFFFFF"
				} else {
					color = "#FF87CEFA"
				}
			}
			b.ChangeAttributes("LineRect", reportsweb.Map{"background": color, "fillEnabled": true, "fillStyle": "Solid", "borderColor": "#FFFFFFFF"}, i)
			for j := 0; j < maxV; j++ {
				if j < 3 && i > len(current) {
					continue
				}
				name := fmt.Sprintf("field%d", j+1)
				o := object(d, name)
				align := "Right"
				if i == 0 {
					align = "Center"
				} else if j == 1 {
					align = "Left"
				} else if j == 0 {
					align = "Center"
				}
				size := 12.0
				if i == 0 {
					size = f(o, "FontSizePt")
				}
				b.SetValueAt(name, "", i, true)
				b.ChangeAttributes(name, reportsweb.Map{"x": xs[j], "width": (f(o, "Width") + adjust[j]) * px, "bold": i == 0, "fontSize": size, "horizontalAlignment": align}, i)
			}
		}
		for j := 0; j <= maxV; j++ {
			x := next
			if j < maxV {
				x = xs[j]
			}
			b.SetValueAt("vLine", "", j, true)
			b.ChangeAttributes("vLine", reportsweb.Map{"x": x}, j)
			if j == 0 || j == maxV {
				b.ChangeAttributes("vLine", reportsweb.Map{"borderWidth": .5 * px}, j)
			}
		}
		for j, label := range []string{"品番", "品名", "数量", "単価", "金額"} {
			b.SetValueAt(fmt.Sprintf("field%d", j+1), label, 0, true)
		}
		total := int64(0)
		for row, r := range current {
			amount := integer(s(r, "数量")) * integer(s(r, "単価"))
			total += amount
			i := row + 1
			b.SetValueAt("field1", s(r, "品番"), i, true)
			b.SetValueAt("field2", s(r, "品名"), i, true)
			b.SetValueAt("field3", s(r, "数量"), i, true)
			b.SetValueAt("field4", number(r["単価"]), i, true)
			b.SetValueAt("field5", number(amount), i, true)
		}
		tax := int64(float64(total)*.05 + .5)
		for k, v := range []struct {
			label  string
			amount int64
		}{{"小計", total}, {"消費税", tax}, {"合計", total + tax}} {
			row := maxH - 3 + k
			b.SetValueAt("field4", v.label, row, true)
			b.SetValueAt("field5", number(v.amount), row, true)
			b.ChangeAttributes("field4", reportsweb.Map{"fontSize": 16, "bold": true, "horizontalAlignment": "Center"}, row)
		}
		b.SetValue("txtTotal", number(total+tax))
		b.ChangeAttributes("hLine", reportsweb.Map{"strokeStyle": "Double"}, maxH-3)
		b.SetValueAt("hLine", "", maxH, true)
		b.ChangeAttributes("hLine", reportsweb.Map{"borderWidth": .5 * px}, maxH)
		b.PageEnd()
	}
	return b, nil
}

func (a *app) buildProducts() (*reportsweb.Builder, error) {
	br, e := a.rows("products", 1)
	if e != nil {
		return nil, e
	}
	sr, e := a.rows("products", 2)
	if e != nil {
		return nil, e
	}
	pr, e := a.rows("products", 3)
	if e != nil {
		return nil, e
	}
	big := map[string]string{}
	small := map[string]string{}
	for _, r := range br {
		big[s(r, "大分類コード")] = s(r, "大分類名称")
	}
	for _, r := range sr {
		small[s(r, "大分類コード")+":"+s(r, "小分類コード")] = s(r, "小分類名称")
	}
	stream := []reportsweb.Map{}
	prevB, prevS := "", ""
	bc, sc := 0, 0
	subtotal := func(kind, name string, count int) reportsweb.Map {
		label := "大分類"
		if kind == "small" {
			label = "小分類"
		}
		return reportsweb.Map{"大分類": "", "小分類": fmt.Sprintf("%s(%s)小計", label, name), "品番": fmt.Sprintf("%d 冊", count), "品名": "", "kind": kind}
	}
	for _, r := range pr {
		bn := big[s(r, "大分類コード")]
		sn := small[s(r, "大分類コード")+":"+s(r, "小分類コード")]
		if prevS != "" && prevS != sn {
			stream = append(stream, subtotal("small", prevS, sc))
			sc = 0
		}
		if prevB != "" && prevB != bn {
			stream = append(stream, subtotal("big", prevB, bc))
			bc = 0
		}
		bv, sv := bn, sn
		if prevB == bn {
			bv = ""
		}
		if prevS == sn {
			sv = ""
		}
		stream = append(stream, reportsweb.Map{"大分類": bv, "小分類": sv, "品番": s(r, "品番"), "品名": s(r, "品名"), "kind": "detail"})
		prevB, prevS = bn, sn
		bc++
		sc++
	}
	if prevS != "" {
		stream = append(stream, subtotal("small", prevS, sc))
	}
	if prevB != "" {
		stream = append(stream, subtotal("big", prevB, bc))
	}
	d, e := a.sampleDefinition("products")
	if e != nil {
		return nil, e
	}
	b := reportsweb.New()
	b.SetDefinition(d)
	for start := 0; start < len(stream); start += 20 {
		end := start + 20
		if end > len(stream) {
			end = len(stream)
		}
		b.PageStart(nil)
		for i, r := range stream[start:end] {
			for _, n := range []string{"大分類", "小分類", "品番", "品名"} {
				b.SetValueAt(n, r[n], i, true)
			}
			for _, n := range []string{"枠_大分類", "枠_小分類", "枠_品番", "枠_品名"} {
				b.SetValueAt(n, "", i, true)
				if r["kind"] != "detail" {
					color := "#FFFFB6C1"
					if r["kind"] == "small" {
						color = "#FFFFFFE0"
					}
					b.ChangeAttributes(n, reportsweb.Map{"background": color, "fillEnabled": true, "fillStyle": "Solid"}, i)
				}
			}
		}
		b.PageEnd()
	}
	return b, nil
}

func s(m reportsweb.Map, k string) string {
	v := m[k]
	if v == nil {
		return ""
	}
	if x, ok := v.(string); ok {
		return x
	}
	if x, ok := v.(float64); ok {
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}
func matching(rows []reportsweb.Map, k, v string) []reportsweb.Map {
	out := []reportsweb.Map{}
	for _, r := range rows {
		if s(r, k) == v {
			out = append(out, r)
		}
	}
	return out
}
func object(d reportsweb.Map, name string) reportsweb.Map {
	for _, raw := range d["Objects"].([]any) {
		o := raw.(map[string]any)
		if s(o, "Name") == name {
			return o
		}
	}
	return reportsweb.Map{}
}
func repeat(o reportsweb.Map) int {
	v := o["Repeat"]
	if v == nil {
		v = o["RepeatCount"]
	}
	return int(num(v))
}
func f(o reportsweb.Map, k string) float64 { return num(o[k]) }
func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		n, _ := x.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(x, 64)
		return n
	}
	return 0
}
func integer(v string) int64 { n, _ := strconv.ParseInt(v, 10, 64); return n }
func number(v any) string {
	n := int64(num(v))
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	x := strconv.FormatInt(n, 10)
	for i := len(x) - 3; i > 0; i -= 3 {
		x = x[:i] + "," + x[i:]
	}
	return sign + x
}
func japaneseDate(v string) string {
	if len(v) >= 10 {
		p := strings.Split(v[:10], "-")
		if len(p) == 3 {
			m, _ := strconv.Atoi(p[1])
			d, _ := strconv.Atoi(p[2])
			return fmt.Sprintf("%s年%d月%d日", p[0], m, d)
		}
	}
	return v
}
func now(japanese bool) string {
	t := time.Now().In(time.FixedZone("JST", 9*3600))
	if japanese {
		return fmt.Sprintf("%d年%d月%d日", t.Year(), t.Month(), t.Day())
	}
	return t.Format("2006/01/02 15:04:05")
}
func orderedValues(m reportsweb.Map) []string {
	return []string{s(m, "郵便番号"), s(m, "市区町村"), s(m, "住所")}
}
func at(v []string, i int) string {
	if i < len(v) {
		return v[i]
	}
	return ""
}
