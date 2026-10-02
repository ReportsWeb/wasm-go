package main

import (
 "encoding/json"
 "io"
 "net/http"
 "os"
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

func main() {
 data, err := printData(); if err != nil { panic(err) }
 engine := reportsweb.NewEngine("")
 http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { if r.URL.Path != "/" { http.NotFound(w, r); return }; http.ServeFile(w, r, "index.html") })
 http.HandleFunc("/print-data", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); _, _ = w.Write(data) })
 http.HandleFunc("/pdf", func(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost { http.Error(w, "POST only", 405); return }
  body, e := io.ReadAll(r.Body); if e != nil { http.Error(w, e.Error(), 400); return }
  // reportsweb.Engine: POST /render/pdf to the Reports.Web engine (REPORTS_ENGINE_URL).
  pdf, e := engine.RenderPDF(r.Context(), body); if e != nil { http.Error(w, e.Error(), 502); return }
  w.Header().Set("Content-Type", "application/pdf"); _, _ = w.Write(pdf)
 })
 http.Handle("/reports.web/", http.StripPrefix("/reports.web/", http.FileServer(http.Dir("/app/reports.web"))))
 http.Handle("/demo/reports.web/", http.StripPrefix("/demo/reports.web/", http.FileServer(http.Dir("/app/reports.web"))))
 if err = http.ListenAndServe(":8080", nil); err != nil { panic(err) }
}
