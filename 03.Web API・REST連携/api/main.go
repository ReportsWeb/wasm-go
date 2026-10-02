package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/ReportsWeb/wasm-go/reportsweb"
	"html"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const routeBase = "/demo/reports.web/samples/go"
const maxBody = 32 << 20

var samples = [][2]string{{"quick-start", "あっという間に帳票出力"}, {"multiples-of-ten", "10のサンプル"}, {"postal", "郵便番号一覧（定義切替）"}, {"estimate", "見積書（表紙＋明細）"}, {"invoice", "請求書"}, {"products", "商品大小分類（途中で小計）"}, {"business-card", "名刺"}, {"design-showcase", "デザイン機能見本"}}

type app struct {
	resources, fixtures, webRoot, template, engine, assetBase, preview, designer string
	db                                                                           *sql.DB
	client                                                                       *http.Client
	render                                                                       chan struct{}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	a := &app{resources: env("REPORTS_RESOURCE_ROOT", "/opt/reports/resources"), fixtures: env("REPORTS_FIXTURE_ROOT", "/opt/reports/fixtures"), webRoot: env("REPORTS_WEB_ROOT", "/opt/reports/web"), engine: strings.TrimRight(env("REPORTS_ENGINE_URL", "http://127.0.0.1:3107"), "/"), assetBase: env("REPORTS_PUBLIC_ASSET_BASE", "http://127.0.0.1:8095"+routeBase+"/api?action=asset&name="), preview: env("REPORTS_PREVIEW_URL", "/demo/reports.web/preview/"), designer: env("REPORTS_DESIGNER_URL", "/demo/reports.web/design/"), client: &http.Client{Timeout: 90 * time.Second}, render: make(chan struct{}, 1)}
	if databaseURL := os.Getenv("REPORTS_DB_URL"); databaseURL != "" {
		var e error
		a.db, e = sql.Open("postgres", databaseURL)
		if e != nil {
			log.Fatal(e)
		}
		if e = a.db.Ping(); e != nil {
			log.Fatal(e)
		}
		defer a.db.Close()
	}
	b, e := os.ReadFile(env("REPORTS_TEMPLATE", "public/index.html"))
	if e != nil {
		log.Fatal(e)
	}
	a.template = string(b)
	m := http.NewServeMux()
	m.HandleFunc("/", a.index)
	m.HandleFunc(routeBase, a.index)
	m.HandleFunc(routeBase+"/", a.index)
	m.HandleFunc(routeBase+"/health", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]string{"status": "UP"}, "application/json", "")
	})
	m.HandleFunc(routeBase+"/api", a.api)
	m.HandleFunc(routeBase+"/api.php", a.api)
	for _, dir := range []string{"preview", "design", "assets", "barcode", "server"} {
		m.Handle("/demo/reports.web/"+dir+"/", http.StripPrefix("/demo/reports.web/"+dir+"/", http.FileServer(http.Dir(filepath.Join(a.webRoot, dir)))))
	}
	for _, file := range []string{"font-map.json", "fontmap.json", "preview-help.html"} {
		m.HandleFunc("/demo/reports.web/"+file, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(a.webRoot, filepath.Base(r.URL.Path)))
		})
	}
	addr := env("HOST", "127.0.0.1") + ":" + env("PORT", "8095")
	log.Printf("Go sample listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, limit(m)))
}
func limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		next.ServeHTTP(w, r)
	})
}
func valid(s string) bool {
	for _, v := range samples {
		if v[0] == s {
			return true
		}
	}
	return false
}
func (a *app) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, routeBase+"/", http.StatusTemporaryRedirect)
		return
	}
	if r.URL.Path != routeBase && r.URL.Path != routeBase+"/" {
		http.NotFound(w, r)
		return
	}
	selected := r.URL.Query().Get("sample")
	if !valid(selected) {
		selected = "invoice"
	}
	var options strings.Builder
	for _, v := range samples {
		sel := ""
		if v[0] == selected {
			sel = " selected"
		}
		fmt.Fprintf(&options, "<option value=\"%s\"%s>%s</option>", v[0], sel, html.EscapeString(v[1]))
	}
	body := strings.ReplaceAll(a.template, "{{SAMPLES}}", options.String())
	body = strings.ReplaceAll(body, "{{PREVIEW_URL}}", html.EscapeString(a.preview))
	d, _ := json.Marshal(a.designer)
	body = strings.ReplaceAll(body, "{{DESIGNER_URL_JSON}}", string(d))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, body)
}
func (a *app) api(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	action := q.Get("action")
	if action == "" {
		action = "data"
	}
	sample := q.Get("sample")
	if sample == "" {
		sample = "invoice"
	}
	if action == "asset" {
		a.asset(w, q.Get("name"))
		return
	}
	if !valid(sample) {
		problem(w, 400, "帳票を選択してください。")
		return
	}
	switch action {
	case "catalog":
		m := map[string]string{}
		for _, v := range samples {
			m[v[0]] = v[1]
		}
		jsonResponse(w, m, "application/json", "")
	case "definition":
		name := sample + ".prepdj"
		if sample == "postal" {
			name = "postal-1.prepdj"
		}
		v, e := readJSON(filepath.Join(a.resources, "definitions", name))
		if e != nil {
			problem(w, 500, e.Error())
			return
		}
		externalize(v, sample, a.assetBase)
		if e = inline(v, a); e != nil {
			problem(w, 500, e.Error())
			return
		}
		jsonResponse(w, v, "application/vnd.pao.reports-definition+json", sample+".prepdj")
	case "data":
		v, e := a.printData(sample)
		if e != nil {
			problem(w, 500, e.Error())
			return
		}
		refresh(v, sample, a.assetBase)
		if e = inline(v, a); e != nil {
			problem(w, 500, e.Error())
			return
		}
		jsonResponse(w, v, "application/vnd.pao.reports-printdata+json", sample+".prepej")
	case "server-pdf":
		if r.Method != "POST" {
			problem(w, 405, "POSTを使用してください。")
			return
		}
		a.pdf(w, r, sample)
	default:
		problem(w, 400, "未対応の操作です。")
	}
}
func (a *app) printData(sample string) (any, error) {
	if a.db == nil {
		return nil, fmt.Errorf("REPORTS_DB_URL is required to build report data")
	}
	return a.catalogPrintData(sample)
}
func readJSON(path string) (any, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var v any
	e = json.Unmarshal(b, &v)
	return v, e
}
func jsonResponse(w http.ResponseWriter, v any, kind, file string) {
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if file != "" {
		w.Header().Set("Content-Disposition", "inline; filename=\""+file+"\"")
	}
	json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func (a *app) asset(w http.ResponseWriter, name string) {
	if name != "kakuin.png" && name != "estimate-header.jpg" {
		problem(w, 400, "未登録の画像資源です。")
		return
	}
	b, e := os.ReadFile(filepath.Join(a.resources, "images", name))
	if e != nil {
		problem(w, 500, e.Error())
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}
func walk(v any, fn func(map[string]any, string, any)) {
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			fn(x, k, c)
			walk(x[k], fn)
		}
	case []any:
		for _, c := range x {
			walk(c, fn)
		}
	}
}
func externalize(v any, sample, asset string) {
	m, _ := v.(map[string]any)
	objects, _ := m["Objects"].([]any)
	for _, raw := range objects {
		o, _ := raw.(map[string]any)
		n, _ := o["Name"].(string)
		file := ""
		if (sample == "invoice" || sample == "estimate") && n == "Image1" {
			file = "kakuin.png"
		}
		if sample == "estimate" && n == "Image2" {
			file = "estimate-header.jpg"
		}
		if file != "" {
			o["ImagePath"] = asset + file
			o["ImageDataBase64"] = ""
		}
	}
}
func refresh(v any, sample, asset string) {
	root, _ := v.(map[string]any)
	externalize(root["Definition"], sample, asset)
	today := time.Now().In(time.FixedZone("JST", 9*3600))
	walk(v, func(m map[string]any, k string, value any) {
		if k == "Value" {
			name, _ := m["Name"].(string)
			if name == "txtDate" {
				m[k] = fmt.Sprintf("%d年%d月%d日", today.Year(), today.Month(), today.Day())
			}
			if sample == "invoice" && name == "Image1" {
				m[k] = asset + "kakuin.png"
			}
		}
	})
	pages, _ := root["Pages"].([]any)
	for _, raw := range pages {
		page, _ := raw.(map[string]any)
		externalize(page["Definition"], sample, asset)
	}
}
func (a *app) pdf(w http.ResponseWriter, r *http.Request, sample string) {
	select {
	case a.render <- struct{}{}:
		defer func() { <-a.render }()
	default:
		problem(w, 429, "PDF作成中です。少し待ってからお試しください。")
		return
	}
	body, e := io.ReadAll(r.Body)
	if e != nil {
		problem(w, 400, "JSON形式が不正です。")
		return
	}
	var v any
	if json.Unmarshal(body, &v) != nil {
		problem(w, 400, "JSON形式が不正です。")
		return
	}
	if e = inline(v, a); e != nil {
		problem(w, 400, e.Error())
		return
	}
	payload, _ := json.Marshal(v)
	// reportsweb.Engine: POST /render/pdf to the Reports.Web engine.
	pdf, e := reportsweb.NewEngine(a.engine).RenderPDF(r.Context(), payload)
	if e != nil {
		problem(w, 502, "サーバーでPDFを作成できませんでした。")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\""+sample+".pdf\"")
	w.Header().Set("X-Reports-Engine", "server-wasm")
	w.Write(pdf)
}
func inline(v any, a *app) error {
	var first error
	walk(v, func(m map[string]any, k string, value any) {
		if first != nil {
			return
		}
		s, ok := value.(string)
		if !ok {
			return
		}
		if strings.HasPrefix(s, a.assetBase) {
			name := strings.TrimPrefix(s, a.assetBase)
			if name != "kakuin.png" && name != "estimate-header.jpg" {
				first = fmt.Errorf("未登録の画像資源です。")
				return
			}
			b, e := os.ReadFile(filepath.Join(a.resources, "images", name))
			if e != nil {
				first = e
				return
			}
			m[k] = "data:" + mime.TypeByExtension(filepath.Ext(name)) + ";base64," + base64.StdEncoding.EncodeToString(b)
		} else if k == "ImagePath" && s != "" && !strings.HasPrefix(s, "data:") {
			first = fmt.Errorf("外部画像は登録した画像か埋め込み画像を使用してください。")
		}
	})
	return first
}
