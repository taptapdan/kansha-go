package render

import (
	"html"
	"html/template"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var (
	goLexer   = chroma.Coalesce(lexers.Get("go"))
	formatter = chromahtml.New(chromahtml.WithClasses(true), chromahtml.PreventSurroundingPre(true))
)

// highlight returns Go source as HTML with chroma's token classes. The colors
// come from web/css/input.css, so they follow light and dark mode.
func highlight(code string) template.HTML {
	it, err := goLexer.Tokenise(nil, code)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(code))
	}
	var b strings.Builder
	if err := formatter.Format(&b, styles.Fallback, it); err != nil {
		return template.HTML(template.HTMLEscapeString(code))
	}
	return template.HTML(b.String())
}

var preRe = regexp.MustCompile(`(?s)<pre>(.*?)</pre>`)

// highlightPre highlights the code blocks (indented lines) inside a rendered
// doc comment. They are almost always Go.
func highlightPre(h string) string {
	return preRe.ReplaceAllStringFunc(h, func(m string) string {
		code := html.UnescapeString(preRe.FindStringSubmatch(m)[1])
		return "<pre>" + string(highlight(code)) + "</pre>"
	})
}
