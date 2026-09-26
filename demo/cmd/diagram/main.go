// Command diagram draws the dependency graph of this project and writes it to
// docs/ in two forms, using the graph module of ArchUnitGo:
//
//   - architecture.mmd  — the raw Mermaid source (ArchUnitGo's ExportAsMermaid
//     terminal). This renders as a picture on GitHub and anywhere else Mermaid
//     is understood.
//   - architecture.html  — the same diagram rendered in a browser. ArchUnitGo's
//     own ExportAsHTML terminal is deliberately script-free and self-contained,
//     so it cannot draw a graph; this page instead wraps ToMermaid's output in a
//     small template that loads mermaid.js from a CDN to lay the arrows out.
//     It needs a network connection, which is fine for a demo.
package main

import (
	"bytes"
	"html/template"
	"log"
	"os"

	archunit "github.com/LukasNiessen/ArchUnitGo"
)

func main() {
	diagram := archunit.ProjectGraph(nil).
		CollapseToFolderDepth(2).
		Titled("the demo's hexagonal architecture")

	if err := diagram.ExportAsMermaid("docs/architecture.mmd"); err != nil {
		log.Fatal(err)
	}

	mermaid, err := diagram.ToMermaid()
	if err != nil {
		log.Fatal(err)
	}
	if err := writeRenderedHTML("docs/architecture.html", mermaid); err != nil {
		log.Fatal(err)
	}

	log.Println("wrote docs/architecture.mmd and docs/architecture.html")
}

// writeRenderedHTML writes a browser-renderable page that lays the Mermaid
// source out with mermaid.js. The diagram text is HTML-escaped by the template,
// which is safe: the browser decodes it before Mermaid reads the element, so the
// picture is identical to what architecture.mmd describes.
func writeRenderedHTML(path, mermaid string) error {
	const page = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>the demo's hexagonal architecture</title>
<script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>
<style>
body { font-family: system-ui, sans-serif; margin: 2rem auto; max-width: 64rem; color: #222; }
h1 { font-size: 1.5rem; }
.mermaid { display: flex; justify-content: center; }
</style>
</head>
<body>
<h1>the demo's hexagonal architecture</h1>
<pre class="mermaid">{{.Diagram}}</pre>
</body>
</html>
`
	tmpl, err := template.New("page").Parse(page)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Diagram string }{Diagram: mermaid}); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
