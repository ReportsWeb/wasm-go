package main

import (
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "os/exec"
 "os/signal"
 "path/filepath"
 "runtime"
 "strings"
 "syscall"
 "time"
 "github.com/ReportsWeb/wasm-go/reportsweb"
)

func printData() ([]byte, error) {
 raw, err := os.ReadFile("definition/quick-report.prepdj"); if err != nil { return nil, err }
 var definition reportsweb.Map; if err = json.Unmarshal(raw, &definition); err != nil { return nil, err }
 report := reportsweb.New(); if err = report.SetDefinition(definition); err != nil { return nil, err }
 if err = report.PageStart(nil); err != nil { return nil, err }
 if err = report.SetValue("Title", "あっという間に帳票出力"); err != nil { return nil, err }
 if err = report.SetValue("CustomerName", "株式会社パオ"); err != nil { return nil, err }
 if err = report.PageEnd(); err != nil { return nil, err }
 return report.JSON(false)
}

// エンジンへの要求は90秒で打ち切る。止まっていれば502、時間切れは504。例外の文字列は応答へ出さない。
var engineClient = &http.Client{Timeout: 90 * time.Second}

func problem(w http.ResponseWriter, status int, message string) {
 w.Header().Set("Content-Type", "application/json; charset=utf-8"); w.WriteHeader(status)
 _ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func main() {
 if _, err := os.Stat("definition/quick-report.prepdj"); err != nil { fmt.Println("definition/quick-report.prepdj が見つかりません。このフォルダー（01.あっという間に帳票出力）で起動してください。"); return }
 reportsWeb := findReportsWeb()
 if reportsWeb == "" { fmt.Println("Runtime/reports.web が見つかりません。ZIP を展開したフォルダーの並びのまま動かしてください。"); return }
 engine := prepareEngine(reportsWeb)
 data, err := printData(); if err != nil { panic(err) }
 http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { if r.URL.Path != "/" { http.NotFound(w, r); return }; http.ServeFile(w, r, "index.html") })
 http.HandleFunc("/print-data", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); _, _ = w.Write(data) })
 http.HandleFunc("/pdf", func(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost { http.Error(w, "POST only", 405); return }
  body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<20))
  if e != nil { var tooLarge *http.MaxBytesError; if errors.As(e, &tooLarge) { problem(w, 413, "印刷データは64MiB以内にしてください。") } else { problem(w, 400, "要求を読み取れませんでした。") }; return }
  pdf, e := reportsweb.NewEngine(engine).RenderPDF(r.Context(), body)
  if e != nil { problem(w, 502, "PDF engine request failed"); return }
  w.Header().Set("Content-Type", "application/pdf"); _, _ = w.Write(pdf)
 })
 http.Handle("/reports.web/", http.StripPrefix("/reports.web/", http.FileServer(http.Dir(reportsWeb))))
 http.Handle("/demo/reports.web/", http.StripPrefix("/demo/reports.web/", http.FileServer(http.Dir(reportsWeb))))
 // Docker の中では 8080 番。外（go run .）では 127.0.0.1 の 19250 番（compose と同じ番号）。
 address := ":8080"
 if !inDocker { port := os.Getenv("PORT"); if port == ""{ port = "19250" }; address = "127.0.0.1:" + port }
 listener, err := net.Listen("tcp", address); if err != nil { stopEngine(); panic(err) }
 if !inDocker {
  url := "http://" + address + "/"
  fmt.Printf("見本の帳票：%s（止めるときは Ctrl+C）\n", url)
  openBrowser(url)
 }
 if err = http.Serve(listener, nil); err != nil { stopEngine(); panic(err) }
}

// Docker を使わずに `go run .` だけで動かすための準備（Docker の中では何もしない）。
// - 製品（Runtime/reports.web）は、ZIP の並びのまま ../../../Runtime/reports.web にある。
// - PDF を作るエンジンは、REPORTS_ENGINE_URL が無ければ 127.0.0.1:3107。居なければ、PC の Node.js で子として起動する。
var inDocker = isFile("/app/reports.web/design/index.html")
var engineProcess *exec.Cmd

func isFile(path string) bool { info, err := os.Stat(path); return err == nil && !info.IsDir() }

func findReportsWeb() string {
 if configured := os.Getenv("REPORTS_WEB_ROOT"); configured != "" { return configured }
 if inDocker { return "/app/reports.web" }
 dir, err := os.Getwd(); if err != nil { return "" }
 for {
  candidate := filepath.Join(dir, "Runtime", "reports.web")
  if isFile(filepath.Join(candidate, "design", "index.html")) { return candidate }
  parent := filepath.Dir(dir); if parent == dir { return "" }; dir = parent
 }
}

func alive(url string) bool {
 client := http.Client{Timeout: 1500 * time.Millisecond}
 response, err := client.Get(url + "/health"); if err != nil { return false }
 response.Body.Close(); return response.StatusCode == 200
}

func prepareEngine(reportsWeb string) string {
 if configured := os.Getenv("REPORTS_ENGINE_URL"); configured != "" { return strings.TrimRight(configured, "/") }
 if inDocker { return "http://engine:3107" }
 url := "http://127.0.0.1:3107"
 if alive(url) { return url }
 node, err := exec.LookPath("node")
 if err != nil { fmt.Println("PDF の保存には Node.js か Docker が要ります（https://nodejs.org/）。プレビューはこのまま使えます。"); return url }
 server := filepath.Join(reportsWeb, "server", "server.mjs")
 engineProcess = exec.Command(node, "--max-old-space-size=1024", server)
 engineProcess.Env = append(os.Environ(), "REPORTS_ENGINE_ROOT="+filepath.Dir(server), "REPORTS_ENGINE_PORT=3107", "REPORTS_ENGINE_HOST=127.0.0.1")
 if err = engineProcess.Start(); err == nil {
  signals := make(chan os.Signal, 1); signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
  go func() { <-signals; stopEngine(); os.Exit(0) }()
  for i := 0; i < 60; i++ { if alive(url) { return url }; time.Sleep(500 * time.Millisecond) }
 }
 fmt.Println("PDF 用のエンジンを起動できませんでした（3107 番が使用中かもしれません）。プレビューはこのまま使えます。")
 return url
}

func stopEngine() { if engineProcess != nil && engineProcess.Process != nil { _ = engineProcess.Process.Kill() } }

// 端末から起動したときだけ、ブラウザーで開く（REPORTS_NO_BROWSER=1 で止められる）。
func openBrowser(url string) {
 if os.Getenv("REPORTS_NO_BROWSER") != "" { return }
 if info, err := os.Stdout.Stat(); err != nil || info.Mode()&os.ModeCharDevice == 0 { return }
 var command *exec.Cmd
 switch runtime.GOOS {
 case "windows": command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
 case "darwin": command = exec.Command("open", url)
 default: command = exec.Command("xdg-open", url)
 }
 _ = command.Start()
}
