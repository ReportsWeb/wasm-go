# reportsweb — Reports.Web for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/ReportsWeb/wasm-go/reportsweb.svg)](https://pkg.go.dev/github.com/ReportsWeb/wasm-go/reportsweb)

**Reports.Web for Go** — 帳票定義（`.prepdj`）に値を入れて印刷データ（PREPEJ）を作り、Reports.Web エンジン（C++ WebAssembly）で PDF にします。請求書・見積書・納品書・宛名ラベル・名刺など、日本の業務帳票向けです。

Build print data (PREPEJ) from a report definition (`.prepdj`) and render it to PDF with the Reports.Web engine (C++ WebAssembly). Designed for Japanese business forms: invoices, estimates, delivery slips, address labels, business cards.

- 製品サイト / Product: https://www.pao.ac/reports.web/
- サンプル / Samples: https://github.com/ReportsWeb/wasm-go
- オンラインデモ / Live demo: https://www.pao.ac/demo/reports.web/samples/go/

## インストール / Install

```sh
go get github.com/ReportsWeb/wasm-go/reportsweb@v1.0.0
docker run -d -p 3107:3107 ghcr.io/reportsweb/engine:1.0.1
```

標準ライブラリーだけを使います（Go 1.22 以降）。エンジンは Docker イメージで動きます。
Standard library only (Go 1.22+). The engine runs as a Docker image.

## 使い方 / Quick start

```go
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/ReportsWeb/wasm-go/reportsweb"
)

func main() {
	raw, _ := os.ReadFile("quick-report.prepdj")
	var definition reportsweb.Map
	_ = json.Unmarshal(raw, &definition)

	b := reportsweb.New()
	_ = b.SetDefinition(definition)
	_ = b.PageStart(nil)
	_ = b.SetValue("Title", "あっという間に帳票出力")
	_ = b.SetValue("CustomerName", "株式会社パオ")
	_ = b.PageEnd()

	pdf, err := reportsweb.NewEngine("http://127.0.0.1:3107").RenderPDF(context.Background(), b)
	if err != nil {
		panic(err)
	}
	_ = os.WriteFile("report.pdf", pdf, 0o644)
}
```

## API

### `Builder`（印刷データ / print data）

| 関数 / Function | 内容 / Description |
|---|---|
| `New()` / `SetDefinition(d)` | 帳票定義（`.prepdj` の内容）を設定 / Set the report definition |
| `PageStart(d)` / `PageEnd()` | ページの開始と終了。`d` にページごとの定義も渡せます / Begin and end a page |
| `SetValue(name, v)` / `SetValueAt(name, v, i, drawing)` | 値を入れる / Set a value |
| `SetRepeatedValue(name, v, x, y, drawing)` | 繰り返し（明細行など）の値 / Value for a repeated object |
| `SetImage(name, v)` / `SetBarcode(name, v)` | 画像・バーコード / Image and barcode |
| `ChangeAttributes(name, values, i)` | 色・位置などの属性 / Change attributes |
| `Value()` / `JSON(pretty)` / `Save(path)` | 印刷データ（PREPEJ）として取り出す・保存する / Get or save the PREPEJ |

### `Engine`

| 関数 / Function | 内容 / Description |
|---|---|
| `NewEngine(url)` | `""` なら環境変数 `REPORTS_ENGINE_URL`、なければ `http://127.0.0.1:3107` |
| `RenderPDF(ctx, data)` | `*Builder`・`Map`・JSON 文字列を受け取り、PDF の `[]byte` を返します |
| `Health(ctx)` | エンジンの状態 / Engine status |

エラーは `*EngineError`（`Status` に HTTP ステータス、つながらない時は 0）で返ります。

## 体験版と製品版 / Trial and product

エンジンのイメージは体験版で、出力には赤い「SAMPLE」の印が付きます。ご購入後に納品するライセンスファイルをエンジンに設定すると、印が消えます。

```sh
docker run -d -p 3107:3107 -v /path/to/reports-web.license:/app/reports-web.license:ro ghcr.io/reportsweb/engine:1.0.1
```

開発用パソコン 1 台につき 1 ライセンス、運用環境はランタイムライセンスフリーです。詳しくは [LICENSE.md](LICENSE.md) と [使用許諾](https://www.pao.ac/reports.web/manual/index.html#16) をご覧ください。

---
Pao@Office — https://www.pao.ac/ — info@pao.ac
