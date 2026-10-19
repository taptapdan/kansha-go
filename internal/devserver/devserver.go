// Package devserver serves the docs locally and rebuilds them whenever a Go
// file, a Markdown file or kansha.yaml changes. Open pages reload themselves
// through a Server-Sent Events connection.
package devserver

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/taptapdan/kansha-go/internal/render"
	"github.com/taptapdan/kansha-go/internal/site"
)

// Server is a running docs server.
type Server struct {
	opts site.Options
	r    *render.Renderer

	mu      sync.RWMutex
	files   *site.MemOutput
	buildID string
	err     error

	clientsMu sync.Mutex
	clients   map[chan string]struct{}
}

// Run builds the site, serves it on addr and rebuilds on changes until ctx is
// canceled.
func Run(ctx context.Context, opts site.Options, addr string) error {
	r, err := render.New()
	if err != nil {
		return err
	}
	opts.Dev = true
	opts.ShowMissing = true
	s := &Server{opts: opts, r: r, clients: map[chan string]struct{}{}}
	s.rebuild()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	root, _ := filepath.Abs(opts.Dir)
	if err := addTree(watcher, root); err != nil {
		return err
	}
	go s.watch(ctx, watcher)

	srv := &http.Server{Addr: addr, Handler: s}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()

	url := addr
	if strings.HasPrefix(url, ":") {
		url = "localhost" + url
	}
	log.Printf("Serving docs at http://%s (Ctrl+C to stop)", url)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) rebuild() {
	start := time.Now()
	out := site.NewMemOutput()
	res, err := site.Build(s.opts, s.r, out)

	s.mu.Lock()
	if err != nil {
		s.err = err
		s.buildID = fmt.Sprintf("error-%d", time.Now().UnixNano())
	} else {
		s.err = nil
		s.files = out
		s.buildID = res.BuildID
	}
	id := s.buildID
	s.mu.Unlock()

	if err != nil {
		log.Printf("Build failed: %v", err)
	} else {
		st := res.Coverage.Stats
		log.Printf("Built %d pages in %s · %d/%d identifiers documented", res.Pages, time.Since(start).Round(time.Millisecond), st.Documented, st.Total)
	}
	s.broadcast(id)
}

// watch rebuilds after relevant files change, waiting briefly so that one save
// (which editors often turn into several events) causes one rebuild.
func (s *Server) watch(ctx context.Context, w *fsnotify.Watcher) {
	var timer *time.Timer
	trigger := make(chan struct{}, 1)
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					addTree(w, ev.Name)
					continue
				}
			}
			if !relevant(ev.Name) || ev.Has(fsnotify.Chmod) && !ev.Has(fsnotify.Write) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(120*time.Millisecond, func() {
				select {
				case trigger <- struct{}{}:
				default:
				}
			})
		case <-trigger:
			s.rebuild()
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Printf("watch: %v", err)
		}
	}
}

func relevant(name string) bool {
	base := filepath.Base(name)
	switch {
	case strings.HasPrefix(base, "."):
		return false
	case base == "kansha.yaml", base == "go.mod":
		return true
	}
	ext := filepath.Ext(base)
	return ext == ".go" || ext == ".md"
}

// addTree watches dir and its subdirectories, skipping the ones the go tool
// ignores (testdata, and names starting with "." or "_") plus node_modules
// and vendor.
func addTree(w *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		name := d.Name()
		if p != dir && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
			name == "testdata" || name == "node_modules" || name == "vendor") {
			return filepath.SkipDir
		}
		return w.Add(p)
	})
}

// ServeHTTP serves the latest build from memory, or the error page if the
// last build failed.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/__kansha/events" {
		s.events(w, req)
		return
	}

	s.mu.RLock()
	files, buildErr, id := s.files, s.err, s.buildID
	s.mu.RUnlock()

	p := strings.TrimPrefix(path.Clean(req.URL.Path), "/")
	if p == "" || p == "." {
		p = "index.html"
	} else if strings.HasSuffix(req.URL.Path, "/") {
		p += "/index.html"
	}

	if buildErr != nil && (path.Ext(p) == ".html" || files == nil) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		errorPage.Execute(w, map[string]string{"Err": buildErr.Error(), "ID": id})
		return
	}
	if files == nil {
		http.NotFound(w, req)
		return
	}

	data, ok := files.Get(p)
	if !ok && path.Ext(p) == "" {
		if _, ok := files.Get(p + "/index.html"); ok {
			http.Redirect(w, req, "/"+p+"/", http.StatusFound)
			return
		}
	}
	if !ok {
		http.NotFound(w, req)
		return
	}
	ctype := mime.TypeByExtension(path.Ext(p))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

// events streams the current build ID; pages reload when it changes.
func (s *Server) events(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")

	ch := make(chan string, 1)
	s.clientsMu.Lock()
	s.clients[ch] = struct{}{}
	s.clientsMu.Unlock()
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, ch)
		s.clientsMu.Unlock()
	}()

	s.mu.RLock()
	id := s.buildID
	s.mu.RUnlock()
	fmt.Fprintf(w, "retry: 500\ndata: %s\n\n", id)
	flusher.Flush()

	for {
		select {
		case <-req.Context().Done():
			return
		case id := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", id)
			flusher.Flush()
		}
	}
}

func (s *Server) broadcast(id string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- id:
		default:
		}
	}
}

var errorPage = template.Must(template.New("error").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Build failed · kansha</title>
<style>
  :root { --bg: #fbfcfc; --fg: #14191c; --muted: #56636a; --line: #e3e8ea; --code: #f5f8f9; --danger: #a8322a; color-scheme: light; }
  @media (prefers-color-scheme: dark) { :root { --bg: #0d1214; --fg: #e4ecef; --muted: #9aa8ae; --line: #1f2a2e; --code: #10181b; --danger: #f08a80; color-scheme: dark; } }
  body { margin: 0; background: var(--bg); color: var(--fg); font: 15px/1.6 ui-sans-serif, system-ui, sans-serif; }
  main { max-width: 48rem; margin: 10vh auto; padding: 0 16px; }
  h1 { color: var(--danger); font-size: 22px; margin: 0 0 8px; }
  p { color: var(--muted); margin: 0 0 16px; }
  pre { background: var(--code); border: 1px solid var(--line); border-radius: 10px; padding: 16px; overflow-x: auto; white-space: pre-wrap; font: 13px/1.6 ui-monospace, Menlo, monospace; }
</style></head>
<body><main>
<h1>The docs couldn't be built</h1>
<p>Fix the error below and save. This page reloads by itself.</p>
<pre>{{.Err}}</pre>
</main>
<script>
var id = "{{.ID}}";
new EventSource("/__kansha/events").onmessage = function (e) { if (e.data && e.data !== id) location.reload(); };
</script>
</body></html>`))
