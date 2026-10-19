# kansha

kansha builds a documentation site for a Go module from the doc comments you
already write. Run it locally to preview the docs with live reload, then publish
the static output to GitHub Pages.

- **No AI and no extra syntax.** Every word on the site comes from standard Go
  doc comments, the same ones `go doc` and pkg.go.dev read.
- **Reminds you to comment.** While previewing, any identifier without a doc
  comment gets a callout showing the comment to add. The Coverage tab and
  `kansha check` list everything that's missing.
- **One page per package**, with an index, types, functions, examples from your
  `_test.go` files, source links and search.
- **Nothing to install besides Go.** The Tailwind CSS is built ahead of time
  and compiled into the binary.

## Install

kansha needs Go 1.26 or newer. From the root of your Go module:

```sh
go get -tool github.com/taptapdan/kansha-go/cmd/kansha
go tool kansha init
```

`go get -tool` records kansha in your `go.mod`, so everyone working on the
project (and CI) uses the same version. `init` adds:

- `kansha.yaml`: optional settings (title, repo URL, branch, what to include)
- `.github/workflows/docs.yml`: publishes the docs to GitHub Pages on every push
- the output folder (`_site/` by default) in `.gitignore`

## Commands

```sh
go tool kansha serve          # preview at http://localhost:4000, rebuilds on save
go tool kansha build          # write the static site to _site/ (or `output` in kansha.yaml)
go tool kansha check          # list identifiers missing doc comments
```

Every command accepts `-C dir` to run against another directory.

| Command | Flags |
| --- | --- |
| `serve` | `-addr localhost:4000` |
| `build` | `-o dir` (overrides `output` in kansha.yaml), `-show-missing` (keep the callouts and Coverage tab in the published site) |
| `check` | exits with status 1 when any doc comment is missing, so it can fail CI |
| `init` | `-force` replaces existing files |

## Publishing to GitHub Pages

1. Run `go tool kansha init` and commit the files it creates.
2. Push to GitHub.
3. In the repository, open **Settings → Pages** and set **Source** to
   **GitHub Actions**.

Each push to your default branch then rebuilds and publishes the docs. All links
in the site are relative, so it works at `https://you.github.io/repo/` without
any base-path setting.

## Writing doc comments

A doc comment is a `//` comment on the lines directly above a declaration, with
no blank line in between. kansha understands the
[standard Go doc comment syntax](https://go.dev/doc/comment).

```go
// Package inventory tracks items in a small shop.
//
// Create items with [NewItem] and change stock with [Restock].
//
// # Usage
//
// Indented lines become a code block:
//
//	it := inventory.NewItem("APL-001", "Apple", 45)
//
// Lists work too:
//   - Quantities never go below zero.
//   - Prices are in cents.
package inventory

// Item is a single product on the shelf.
type Item struct {
	SKU string // Stock keeping unit, e.g. "APL-001".
	Qty int    // Units on the shelf right now.
}

// ParseSKU normalizes a SKU.
//
// Deprecated: use sku.Parse instead.
func ParseSKU(s string) string { ... }

// BUG(dan): ParseSKU accepts lowercase letters.
```

| You write | The site shows |
| --- | --- |
| `// Package name ...` above `package name` (usually in `doc.go`) | The package overview |
| `// Name ...` above a func, type, method, const or var | Its description |
| `# Heading` in a comment | A heading, also listed under "On this page" |
| Indented lines | A highlighted code block |
| `- item` or `1. item` | A list |
| `[Name]`, `[Type.Method]`, `[fmt.Println]` | A link to that identifier |
| A paragraph starting `Deprecated:` | A Deprecated badge and callout |
| A comment after a struct field | The field's description in the Fields table |
| `func ExampleName()` in a `_test.go` file, with `// Output:` | A collapsible example with its output |
| `// BUG(who): ...` | An entry under Known bugs |

By Go convention a comment starts with the name it describes, e.g.
`// Restock adds ...`. kansha shows a small tip when one doesn't.

### What gets documented

Library packages show exported identifiers (names that start with a capital
letter). Commands (`package main`) show everything, since they have no exported
API. Set `unexported: true` in `kansha.yaml` to show unexported identifiers in
library packages too.

## Configuration

Every field in `kansha.yaml` is optional:

```yaml
title: go-tutorial                                # defaults to the module name
description: Notes from learning Go               # shown on the home page when there's no README
repo: https://github.com/taptapdan/go-tutorial    # defaults to the "origin" remote
branch: main                                      # branch for source links
output: _site                                     # where kansha build writes the site
unexported: false                                 # show unexported identifiers in library packages
exclude: [internal/scratch]                       # package directories to skip
```

The output folder is chosen in this order: the `-o` flag, then `output` in
`kansha.yaml`, then `_site`. The generated GitHub workflow reads the folder
from kansha, so changing `output` doesn't break publishing.

If the module has a `README.md`, it becomes the home page.

## Developing kansha

```
cmd/kansha/              CLI entry point
internal/
  config/                kansha.yaml loading
  extract/               go/packages + go/doc → data model
  coverage/              missing-comment detection
  render/                comments → HTML, syntax highlighting, templates
  site/                  builds every page, to disk or memory
  devserver/             kansha serve: file watching and live reload
  scaffold/              kansha init
web/                     templates, Tailwind source, built CSS, JavaScript
testdata/shop/           sample module used by the tests and `just demo`
```

```sh
just test    # run the tests
just demo    # preview the sample module
just css     # rebuild web/dist/kansha.css (needs Node for npx)
```

Run `just css` after changing classes in templates or JavaScript, and commit the
rebuilt CSS. That way people who install kansha don't need Node.
