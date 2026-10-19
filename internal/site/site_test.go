package site

import (
	"strings"
	"testing"

	"github.com/taptapdan/kansha-go/internal/render"
)

func TestBuildShop(t *testing.T) {
	r, err := render.New()
	if err != nil {
		t.Fatal(err)
	}
	out := NewMemOutput()
	res, err := Build(Options{Dir: "../../testdata/shop", ShowMissing: true}, r, out)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pages != 4 {
		t.Errorf("built %d pages, want 4", res.Pages)
	}

	page, ok := out.Get("api/inventory/index.html")
	if !ok {
		t.Fatal("no page for inventory")
	}
	html := string(page)
	for _, want := range []string{
		`<span class="font-mono text-[0.8em] font-medium text-faint">package</span> inventory`,
		`id="Item.Total"`,
		`Restock has no doc comment`,
		`href="#NewItem"`,                       // doc link to a symbol on the same page
		`href="https://pkg.go.dev/fmt#Sprintf"`, // doc link to the standard library
		`id="hdr-usage"`,                        // # Usage heading
		`Deprecated:`,
		`// contains filtered or unexported fields`,
		`href="../../assets/kansha.css"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("inventory page is missing %q", want)
		}
	}

	for _, name := range []string{"index.html", "coverage/index.html", "api/cmd/shopctl/index.html", "search.json", "assets/kansha.css", "assets/kansha.js", ".nojekyll"} {
		if _, ok := out.Get(name); !ok {
			t.Errorf("missing %s", name)
		}
	}
}

func TestBuildHidesMissingByDefault(t *testing.T) {
	r, err := render.New()
	if err != nil {
		t.Fatal(err)
	}
	out := NewMemOutput()
	if _, err := Build(Options{Dir: "../../testdata/shop"}, r, out); err != nil {
		t.Fatal(err)
	}
	if cmd, _ := out.Get("api/cmd/shopctl/index.html"); strings.Contains(string(cmd), "go install") {
		t.Error("command page shows go install although show_install is off")
	}
	page, _ := out.Get("api/inventory/index.html")
	if strings.Contains(string(page), "has no doc comment") {
		t.Error("published build shows missing-doc callouts")
	}
	if _, ok := out.Get("coverage/index.html"); ok {
		t.Error("published build includes the coverage page")
	}
}

func TestDirOutputRebuild(t *testing.T) {
	dir := t.TempDir() + "/out"
	for i := 0; i < 2; i++ {
		o := DirOutput{Dir: dir}
		if err := o.Prepare(); err != nil {
			t.Fatalf("build %d: %v", i+1, err)
		}
		if err := o.Write("index.html", []byte("hi")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDirOutputRefusesForeignDir(t *testing.T) {
	dir := t.TempDir()
	if err := (DirOutput{Dir: dir}).Write("notes.txt", []byte("mine")); err != nil {
		t.Fatal(err)
	}
	if err := (DirOutput{Dir: dir}).Prepare(); err == nil {
		t.Error("Prepare cleared a folder kansha didn't create")
	}
}
