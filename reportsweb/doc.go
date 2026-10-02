// Package reportsweb is the Reports.Web client for Go.
//
// Build print data (PREPEJ) from a report definition (.prepdj) designed in the
// Reports.Web browser designer, then render it to PDF with the Reports.Web
// engine (C++ WebAssembly, Docker image ghcr.io/reportsweb/engine).
//
//	b := reportsweb.New()
//	_ = b.SetDefinition(definition)
//	_ = b.PageStart(nil)
//	_ = b.SetValue("Title", "あっという間に帳票出力")
//	_ = b.PageEnd()
//	pdf, err := reportsweb.NewEngine("").RenderPDF(ctx, b)
//
// The engine image is a trial build: output carries a red "SAMPLE" mark until
// the license file delivered on purchase is configured.
// Product: https://www.pao.ac/reports.web/
package reportsweb
