// Package web holds the site's templates, CSS and JavaScript, compiled into
// the kansha binary so projects that use it need no Node or npm.
//
// The CSS in dist/ is generated from css/input.css by Tailwind. Rebuild it
// with `just css` after changing templates or styles.
package web

import "embed"

// FS contains templates/, dist/ and js/.
//
//go:embed templates dist js
var FS embed.FS
