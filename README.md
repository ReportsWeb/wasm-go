<p align="center"><img src="docs/images/banner.png" alt="Reports.Web for Go" width="100%"></p>

# Reports.Web — Go サンプル

[![Go Reference](https://pkg.go.dev/badge/github.com/ReportsWeb/wasm-go/reportsweb.svg)](https://pkg.go.dev/github.com/ReportsWeb/wasm-go/reportsweb)
[![engine](https://img.shields.io/badge/docker-ghcr.io%2Freportsweb%2Fengine-2563eb)](https://github.com/ReportsWeb/engine)
[![Live demo](https://img.shields.io/badge/demo-www.pao.ac-ffd166)](https://www.pao.ac/demo/reports.web/samples/go/)

**Reports.Web** は、ブラウザで帳票をデザインし、いろいろな言語から PDF を作る帳票ツールです。このリポジトリは、**Go** から使うサンプル（配布 ZIP に入っているものと同じソース）です。

請求書・見積書・納品書・宛名ラベル・名刺など、日本の業務帳票をそのまま作れます。帳票のレイアウトはブラウザのデザイナーで作り、アプリは値を入れて「印刷データ（PREPEJ）」を作るだけです。描画は C++ WebAssembly の帳票エンジンが行います。

- 製品サイト：https://www.pao.ac/reports.web/
- オンラインデモ：https://www.pao.ac/demo/reports.web/samples/go/
- Go モジュール：[`github.com/ReportsWeb/wasm-go/reportsweb`](reportsweb/README.md)
- 帳票エンジン（Docker）：[`ghcr.io/reportsweb/engine`](https://github.com/ReportsWeb/engine)

| デザイナー（ブラウザ） | Go のサンプル（プレビュー） |
|---|---|
| <img src="docs/images/designer.png" alt="Reports.Web Designer"> | <img src="docs/images/go-demo.png" alt="Go sample"> |

## サンプルを選ぶ

| # | フォルダー | 内容 | ポート |
|:-:|---|---|---|
| 01 | [01.あっという間に帳票出力](01.あっという間に帳票出力/README.md) | Go の短いコードで、1 ページの帳票を表示・PDF 出力 | http://127.0.0.1:19250/ |
| 02 | [02.全帳票をWebでプレビュー](02.全帳票をWebでプレビュー/README.md) | Go と PostgreSQL で 8 種類の帳票（請求書・見積書・郵便番号 81 ページなど）を作る Web アプリ | http://127.0.0.1:19251/ |
| 03 | [03.Web API・REST連携](03.Web%20API・REST連携/README.md) | 02 と同じ帳票処理を Web API にし、別の利用側 Web アプリから呼ぶ | http://127.0.0.1:19252/ |

迷ったら 01 から順にどうぞ。各フォルダーは独立していて、別々に起動・停止できます。

## クイックスタート

必要なのは Docker（Docker Desktop など）だけです。Go や PostgreSQL をパソコンに入れる必要はありません。

```sh
git clone https://github.com/ReportsWeb/wasm-go.git
cd "wasm-go/01.あっという間に帳票出力"
docker compose up -d --build --wait --wait-timeout 180
# ブラウザで http://127.0.0.1:19250/ を開く
docker compose down --volumes   # 終わったら停止・削除
```

## コードの要点

```go
import "github.com/ReportsWeb/wasm-go/reportsweb"

b := reportsweb.New()
_ = b.SetDefinition(definition)          // ブラウザのデザイナーで作った .prepdj
_ = b.PageStart(nil)
_ = b.SetValue("Title", "あっという間に帳票出力")
_ = b.PageEnd()

pdf, err := reportsweb.NewEngine("http://engine:3107").RenderPDF(ctx, b)
```

## しくみ

```
Go アプリ ──(印刷データ PREPEJ / JSON)──▶ 帳票エンジン ghcr.io/reportsweb/engine ──▶ PDF
    ▲                                          （C++ WebAssembly、Docker）
帳票定義 .prepdj（ブラウザのデザイナーで作成）
```

各サンプルの `compose.yaml` は、帳票エンジンのイメージを起動し、同じイメージからデザイナー・プレビュー画面のファイルを取り出して使います。

## 体験版と製品版

帳票エンジンは**体験版**で、出力には赤い「SAMPLE」の印が付きます。ご購入後に納品するライセンスファイルを帳票エンジンに設定すると印が消えます（開発用パソコン 1 台につき 1 ライセンス、運用環境はランタイムライセンスフリー）。

- ご購入・お見積もり：https://www.pao.ac/reports.web/buy.html
- 使用許諾：https://www.pao.ac/reports.web/manual/index.html#16

このリポジトリのサンプルコードは [MIT License](LICENSE) です。Go モジュール `reportsweb/` と帳票エンジンには Reports.Web の使用許諾（[reportsweb/LICENSE.md](reportsweb/LICENSE.md)）が適用されます。

## ほかの言語

同じ帳票定義・印刷データを、PHP・Java・.NET・Go・Ruby・Rust・Node.js から使えます。Pure Java・WPF 版もあります。→ https://github.com/ReportsWeb

`Manual/` のプログラマーズガイドは配布 ZIP 向けに書かれています。このリポジトリでは、印刷データを作る部品を同梱の `printdata` ではなく Go モジュール `reportsweb` から読み込みます。

---

## English

**Reports.Web** is a report designer and PDF engine. Design report layouts (invoices, estimates, delivery slips, address labels, business cards) in the browser, then build print data (PREPEJ JSON) from your application and render PDF with the C++ WebAssembly engine. This repository contains the **Go** samples — the same source as the distributed trial ZIP.

- Module: [`github.com/ReportsWeb/wasm-go/reportsweb`](reportsweb/README.md) (`go get github.com/ReportsWeb/wasm-go/reportsweb@v1.0.0`)
- Engine: [`ghcr.io/reportsweb/engine`](https://github.com/ReportsWeb/engine) (Docker)
- Live demo: https://www.pao.ac/demo/reports.web/samples/go/

Quick start: install Docker, then

```sh
git clone https://github.com/ReportsWeb/wasm-go.git
cd "wasm-go/01.あっという間に帳票出力"
docker compose up -d --build --wait --wait-timeout 180   # open http://127.0.0.1:19250/
```

The engine is a trial build; output carries a red "SAMPLE" mark until a purchased license file is configured. Sample code: MIT. Module and engine: Reports.Web license.

Pao@Office — https://www.pao.ac/ — info@pao.ac
